// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !math_big_pure_go

package big // import "github.com/cymertek/go-big"

import "golang.org/x/sys/cpu"

var support_adx = cpu.X86.HasADX && cpu.X86.HasBMI2
