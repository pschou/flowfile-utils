// Copyright 2021 github.com/pschou/go-iothrottler
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package iothrottler

import (
	"time"
)

// IOThrottler struct with a channel to release packets at a given rate
type Limit struct {
	C             chan int8
	Bandwidth, fs int
	t             time.Duration
	run           bool
}

// Create a new Limit with specified bandwith limitation
func NewLimit(Bandwidth, MTU, frameSpec int) (t *Limit) {
	t = &Limit{
		Bandwidth: Bandwidth,
		fs:        frameSpec,
		C:         make(chan int8),
		t:         time.Second * 8 * time.Duration(MTU+frameSpec) / time.Duration(Bandwidth),
		run:       true,
	}
	go func(iot *Limit) {
		//fmt.Println("per frame:", t.t)
		var now, next time.Time
		var step time.Duration
		next = time.Now().Add(iot.t)
		for iot.run {
			iot.C <- 1
			now = time.Now()
			step = next.Sub(now)
			if step > 0 {
				time.Sleep(step)
				next = next.Add(iot.t)
			} else {
				next = now.Add(iot.t)
			}
		}
	}(t)
	return
}

// Gradually affect the MTU
func (t *Limit) SkewMTU(MTU int) {
	t.t = (11*t.t + time.Second*8*time.Duration(MTU+26+t.fs)/time.Duration(t.Bandwidth)) / 12
}

// Set the MTU size
func (t *Limit) SetMTU(MTU int) {
	t.t = time.Second * 8 * time.Duration(MTU+26+t.fs) / time.Duration(t.Bandwidth)
}

// Stop and close the channel
func (t *Limit) Stop() {
	t.run = false
}
