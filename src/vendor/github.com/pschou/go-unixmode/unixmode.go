// Copyright 2023 github.com/pschou/go-unixmode
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

// UnixMode is a UNIX file POSIX FileMode utility.  This module provides the
// ability to work with the GoLang built-in fs.FileMode on a Unix/Linux system
// and preserve the bits as needed to update file modes, read file modes, or
// send file modes over the wire in a POSIX compliant format.
//
// To get a file mode in string format:
//
//   stat, _ := os.Lstat("/tmp")
//   m := stat.FileMode()
//   fmt.Printf("mode: %q\n", unixmode.FileModeString(m))
//
// Conversely, to set a file permissions using UnixMode:
//
//   m := unixmode.Parse("r-xr-sr--")
//   os.Chmod("myfile", m.Perm())
//
// Which will return, "drwxrwxrwt "
//
// UnixMode.String emulates the filemodestring - by filling in string STR with
// an ls-style ASCII representation of the st_mode field of file stats block STATP.
// 12 characters are stored in STR.
//
// The characters stored in STR are:
// 0    File type, as in TypeLetter
// 1    'r' if the owner may read, '-' otherwise.
// 2    'w' if the owner may write, '-' otherwise.
// 3    'x' if the owner may execute, 's' if the file is
//      set-user-id, '-' otherwise.
//      'S' if the file is set-user-id, but the execute
//      bit isn't set.
// 4    'r' if group members may read, '-' otherwise.
// 5    'w' if group members may write, '-' otherwise.
// 6    'x' if group members may execute, 's' if the file is
//      set-group-id, '-' otherwise.
//      'S' if it is set-group-id but not executable.
// 7    'r' if any user may read, '-' otherwise.
// 8    'w' if any user may write, '-' otherwise.
// 9    'x' if any user may execute, 't' if the file is "sticky"
//      (will be retained in swap space after execution), '-'
//      otherwise.
//      'T' if the file is sticky but not executable.
// 10   ' ' for compatibility with 4.4BSD strmode,
//      since this interface does not support ACLs.
//
// The TypeLetter functions return a character indicating the type of file
// described by file mode BITS:
//
// - '-' regular file
// - 'b' block special file
// - 'c' character special file
// - 'C' high performance ("contiguous data") file***
// - 'd' directory
// - 'D' door***
// - 'l' symbolic link
// - 'm' multiplexed file (7th edition Unix; obsolete)***
// - 'n' network special file (HP-UX)***
// - 'p' fifo (named pipe)
// - 'P' port***
// - 's' socket
// - 'w' whiteout (4.4BSD)***
// - '?' some other file type
//
// Note: ** = not implemented by GoLang

package unixmode

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// A Mode represents a file's mode and permission bits.
// The bits are used in the POSIX definition.
type Mode uint16

// The defined file mode bits are the most significant bits of the Mode.
// The values of these bits are defined by the Unix filemode standard and
// may be used in wire protocols or disk representations: they must not be
// changed, although new bits might be added.
const (
	// These masks and bits can be used for determining set bits.
	// The Mode(uint16) has all the bit positions utilized 4+3+9=16.
	// These are defined in lstat.c.  Also found in os/types.go as S_*.

	// Types (upper 4 bits)
	ModeTypeMask   Mode = 0170000 /* type of file mask */
	ModeNamedPipe  Mode = 0010000 /* named pipe (fifo) */
	ModeCharDevice Mode = 0020000 /* character special */
	ModeDir        Mode = 0040000 /* directory */
	ModeDevice     Mode = 0060000 /* block special */
	ModeRegular    Mode = 0100000 /* regular */
	ModeSymlink    Mode = 0120000 /* symbolic link */
	ModeSocket     Mode = 0140000 /* socket */

	// Set ID / Sticky (middle 3 bits)
	ModeSetuid Mode = 0004000 /* set-user-ID on execution */
	ModeSetgid Mode = 0002000 /* set-group-ID on execution */
	ModeSticky Mode = 0001000 /* save swapped text even after use */

	// Permissions (lower 9 bits)
	ModeUserMask   Mode = 0000700 /* RWX mask for owner */
	ModeReadUser   Mode = 0000400 /* R for owner */
	ModeWriteUser  Mode = 0000200 /* W for owner */
	ModeExecUser   Mode = 0000100 /* X for owner */
	ModeGroupMask  Mode = 0000070 /* RWX mask for group */
	ModeReadGroup  Mode = 0000040 /* R for group */
	ModeWriteGroup Mode = 0000020 /* W for group */
	ModeExecGroup  Mode = 0000010 /* X for group */
	ModeOtherMask  Mode = 0000007 /* RWX mask for other */
	ModeReadOther  Mode = 0000004 /* R for other */
	ModeWriteOther Mode = 0000002 /* W for other */
	ModeExecOther  Mode = 0000001 /* X for other */

)

// Return the TypeLetter defined in the FileMode bits.
func FileModeTypeLetter(m fs.FileMode) byte {
	switch m & fs.ModeType {
	/* These are the most common.  */
	case 0:
		return '-'
	case fs.ModeDir:
		return 'd'

	/* Other letters standardized by POSIX 1003.1-2004.  */
	case fs.ModeDevice:
		return 'b'
	case fs.ModeCharDevice | fs.ModeDevice:
		return 'c'
	case fs.ModeSymlink:
		return 'l'
	case fs.ModeNamedPipe:
		return 'p'

	/* Other file types (though not letters) standardized by POSIX.  */
	case fs.ModeSocket:
		return 's'

		//  /* Nonstandard file types.  */
		//  if (S_ISCTG (bits))
		//    return 'C';
		//  if (S_ISDOOR (bits))
		//    return 'D';
		//  if (S_ISMPB (bits) || S_ISMPC (bits) || S_ISMPX (bits))
		//    return 'm';
		//  if (S_ISNWK (bits))
		//    return 'n';
		//  if (S_ISPORT (bits))
		//    return 'P';
		//  if (S_ISWHT (bits))
		//    return 'w';
	}

	fmt.Printf("mode %0o\n", m&fs.ModeType)
	return '?'
}

// Return the TypeLetter defined in the Mode bits.
func (m Mode) TypeLetter() byte {
	switch m & ModeTypeMask {
	/* These are the most common, so test for them first.  */
	case ModeRegular:
		return '-'
	case ModeDir:
		return 'd'

	/* Other letters standardized by POSIX 1003.1-2004.  */
	case ModeDevice:
		return 'b'
	case ModeCharDevice:
		return 'c'
	case ModeSymlink:
		return 'l'
	case ModeNamedPipe:
		return 'p'

	/* Other file types (though not letters) standardized by POSIX.  */
	case ModeSocket:
		return 's'
	}

	//  /* Nonstandard file types.  */
	//  if (S_ISCTG (bits))
	//    return 'C';
	//  if (S_ISDOOR (bits))
	//    return 'D';
	//  if (S_ISMPB (bits) || S_ISMPC (bits) || S_ISMPX (bits))
	//    return 'm';
	//  if (S_ISNWK (bits))
	//    return 'n';
	//  if (S_ISPORT (bits))
	//    return 'P';
	//  if (S_ISWHT (bits))
	//    return 'w';

	fmt.Printf("mode %0o\n", m&ModeTypeMask)
	return '?'
}

// Return the Mode with the TypeLetter + PermString + " "
// The extra space is for compatibility with 4.4BSD strmode
func FileModeString(m fs.FileMode) string {
	var buf [11]byte
	buf[0] = FileModeTypeLetter(m)
	setIf(&buf[1], m&(1<<8) != 0, 'r', '-')
	setIf(&buf[2], m&(1<<7) != 0, 'w', '-')
	setIfIf(&buf[3], m&(1<<6) != 0, m&fs.ModeSetuid != 0, 's', 'S', 'x', '-')
	setIf(&buf[4], m&(1<<5) != 0, 'r', '-')
	setIf(&buf[5], m&(1<<4) != 0, 'w', '-')
	setIfIf(&buf[6], m&(1<<3) != 0, m&fs.ModeSetgid != 0, 's', 'S', 'x', '-')
	setIf(&buf[7], m&(1<<2) != 0, 'r', '-')
	setIf(&buf[8], m&(1<<1) != 0, 'w', '-')
	setIfIf(&buf[9], m&(1<<0) != 0, m&fs.ModeSticky != 0, 't', 'T', 'x', '-')
	buf[10] = ' '
	return string(buf[:])
}

// Return the lower 12 bits in a UNIX permission string format
func FileModePermString(m fs.FileMode) string {
	var buf [9]byte
	setIf(&buf[0], m&(1<<8) != 0, 'r', '-')
	setIf(&buf[1], m&(1<<7) != 0, 'w', '-')
	setIfIf(&buf[2], m&(1<<6) != 0, m&fs.ModeSetuid != 0, 's', 'S', 'x', '-')
	setIf(&buf[3], m&(1<<5) != 0, 'r', '-')
	setIf(&buf[4], m&(1<<4) != 0, 'w', '-')
	setIfIf(&buf[5], m&(1<<3) != 0, m&fs.ModeSetgid != 0, 's', 'S', 'x', '-')
	setIf(&buf[6], m&(1<<2) != 0, 'r', '-')
	setIf(&buf[7], m&(1<<1) != 0, 'w', '-')
	setIfIf(&buf[8], m&(1<<0) != 0, m&fs.ModeSticky != 0, 't', 'T', 'x', '-')
	return string(buf[:])
}

// Return the Mode with the TypeLetter + PermString + " "
// The extra space is for compatibility with 4.4BSD strmode
func (m Mode) String() string {
	var buf [11]byte
	buf[0] = m.TypeLetter()
	setIf(&buf[1], m&(1<<8) != 0, 'r', '-')
	setIf(&buf[2], m&(1<<7) != 0, 'w', '-')
	setIfIf(&buf[3], m&(1<<6) != 0, m&(1<<11) != 0, 's', 'S', 'x', '-')
	setIf(&buf[4], m&(1<<5) != 0, 'r', '-')
	setIf(&buf[5], m&(1<<4) != 0, 'w', '-')
	setIfIf(&buf[6], m&(1<<3) != 0, m&(1<<10) != 0, 's', 'S', 'x', '-')
	setIf(&buf[7], m&(1<<2) != 0, 'r', '-')
	setIf(&buf[8], m&(1<<1) != 0, 'w', '-')
	setIfIf(&buf[9], m&(1<<0) != 0, m&(1<<9) != 0, 't', 'T', 'x', '-')
	buf[10] = ' '
	return string(buf[:])
}

// Return the lower 12 bits in a UNIX permission string format
func (m Mode) PermString() string {
	var buf [9]byte
	setIf(&buf[0], m&(1<<8) != 0, 'r', '-')
	setIf(&buf[1], m&(1<<7) != 0, 'w', '-')
	setIfIf(&buf[2], m&(1<<6) != 0, m&(1<<11) != 0, 's', 'S', 'x', '-')
	setIf(&buf[3], m&(1<<5) != 0, 'r', '-')
	setIf(&buf[4], m&(1<<4) != 0, 'w', '-')
	setIfIf(&buf[5], m&(1<<3) != 0, m&(1<<10) != 0, 's', 'S', 'x', '-')
	setIf(&buf[6], m&(1<<2) != 0, 'r', '-')
	setIf(&buf[7], m&(1<<1) != 0, 'w', '-')
	setIfIf(&buf[8], m&(1<<0) != 0, m&(1<<9) != 0, 't', 'T', 'x', '-')
	return string(buf[:])
}

func setIfIf(c *byte, test1, test2 bool, tt, tf, ft, ff byte) {
	if test2 {
		if test1 {
			*c = tt
		} else {
			*c = tf
		}
	} else {
		if test1 {
			*c = ft
		} else {
			*c = ff
		}
	}
}
func setIf(c *byte, test bool, t, f byte) {
	if test {
		*c = t
	} else {
		*c = f
	}
}

// Parse will take three formats and convert them into a Mode with the bits set:
//
// "rwsrwxrwx"   - 9  bytes, Returns the lower 12 bits set
//
// "-rwsrwxrwx"  - 10 bytes, Lower 12 bits and includes setting the file ModeType
//
// "-rwsrwxrwx " - 11 bytes, Compatibility with newer os's with ACLs and SELinux contexts
func Parse(in string) (Mode, error) {
	var m Mode
	switch len(in) {
	case 9: // Assume a file and only parse the lower bits
		in = "-" + in
	case 10, 11: // For compatibility with 4.4BSD strmode
		switch in[0] {
		case '-':
			m = m | ModeRegular
		case 'd':
			m = m | ModeDir
		case 'c':
			m = m | ModeCharDevice
		case 'b':
			m = m | ModeDevice
		case 'l':
			m = m | ModeSymlink
		case 'p':
			m = m | ModeNamedPipe
		case 's':
			m = m | ModeSocket
		default:
			return 0, ErrorMode
		}
	default:
		return 0, ErrorModeLength
	}

	var err []string
	setBitIf(&m, &err, in, 1, 'r', ModeReadUser)
	setBitIf(&m, &err, in, 2, 'w', ModeWriteUser)
	setBitIfIf(&m, &err, in, 3, 's', 'S', 'x', ModeSetuid, ModeExecUser)
	setBitIf(&m, &err, in, 4, 'r', ModeReadGroup)
	setBitIf(&m, &err, in, 5, 'w', ModeWriteGroup)
	setBitIfIf(&m, &err, in, 6, 's', 'S', 'x', ModeSetgid, ModeExecGroup)
	setBitIf(&m, &err, in, 7, 'r', ModeReadOther)
	setBitIf(&m, &err, in, 8, 'w', ModeWriteOther)
	setBitIfIf(&m, &err, in, 9, 't', 'T', 'x', ModeSticky, ModeExecOther)
	if len(err) == 0 {
		return m, nil
	}
	return 0, errors.New(strings.Join(err, ","))
}

// A function to parse a fs.FileMode string into the standard fs.FileMode
func ParseFileMode(in string) (fs.FileMode, error) {
	var m fs.FileMode
	for _, c := range in[:len(in)-9] {
		switch c {
		case 'd':
			m |= fs.ModeDir
		case 'a':
			m |= fs.ModeAppend
		case 'l':
			m |= fs.ModeExclusive
		case 'T':
			m |= fs.ModeTemporary
		case 'L':
			m |= fs.ModeSymlink
		case 'D':
			m |= fs.ModeDevice
		case 'p':
			m |= fs.ModeNamedPipe
		case 'S':
			m |= fs.ModeSocket
		case 'u':
			m |= fs.ModeSetuid
		case 'g':
			m |= fs.ModeSetgid
		case 'c':
			m |= fs.ModeCharDevice
		case 't':
			m |= fs.ModeSticky
		case '?':
			m |= fs.ModeIrregular
		default:
			return 0, ErrorMode
		}
	}
	const rwx = "rwxrwxrwx"
	for i, c := range in[len(in)-9:] {
		switch byte(c) {
		case rwx[i]:
			m |= 1 << (8 - i)
		case '-':
		default:
			return 0, ErrorMode
		}
	}
	return m, nil
}

func setBitIf(m *Mode, err *[]string, in string, strPos int, t byte, bitPos Mode) {
	switch in[strPos] {
	case t:
		*m = *m | bitPos
	case '-':
	default:
		*err = append(*err, fmt.Sprintf("Invalid %q at position %d", in[strPos], strPos))
	}
}
func setBitIfIf(m *Mode, err *[]string, in string, strPos int, tt, tf, ft byte, bitPos1, bitPos2 Mode) {
	switch in[strPos] {
	case tt:
		*m = *m | bitPos1 | bitPos2
	case tf:
		*m = *m | bitPos1
	case ft:
		*m = *m | bitPos2
	case '-':
	default:
		*err = append(*err, fmt.Sprintf("Invalid %q at position %d", in[strPos], strPos))
	}
}

var (
	ErrorModeLength = errors.New("Invalid Mode Length")
	ErrorMode       = errors.New("Invalid Mode")
)

// IsDir reports whether m describes a directory.
// That is, it tests for the ModeDir bit being set in m.
func (m Mode) IsDir() bool {
	return m&ModeDir != 0
}

// IsRegular reports whether m describes a regular file.
// That is, it tests that the ModeRegular bit is the only type set.
func (m Mode) IsRegular() bool {
	return m&ModeTypeMask == ModeRegular
}

// Perm returns the Unix permission bits in m.  That is, it returns the
// Mode & 07777, as the lower 12 bits describe the full permissions.
func (m Mode) Perm() Mode {
	return m & 07777
}

// Perm returns the Unix permission bits in FileMode m.  That is, it takes
// the GoLang fs.FileMode and changes the bit flags into the Mode permission
// format without maintaining the type.
//
// This is useful for setting permissions on a linux system, like this:
//
// Similarly, one can achieve the same result by doing:
//   m, err := os.Stat("/tmp")
//   myPerm := unixmode.FileModePerm(m.FileMode())
//   os.Chmod(name, myPerm)
//
// Note: GoLang reserves bits 10-12 along with 3 additional random bit
// locations for sticky bit values.  By keeping 10-12 clear and parsing down
// the higher order bits on top of these three bits, it has effectively
// disabled the lower 3 bits from being used.  This methodology is to create
// "Go's portable mode bits"
//
// This seems like a duplication of efforts for an extra 3 higher order bits to
// be assigned when the lower 3 bits are used when calling os.Chmod and
// conversely the syscallMode.  This duplication of bit usage makes life easier
// for developers only interested in the lower 12, however creating an
// additional higher order bits just seems odd when the lower bits are still
// used.  One may ask "why not re-use bits which already have a pretty stable
// definition?"  With 32 bits in fs.FileMode and twice the declaration of what
// defines a sticky bit, they would have a valid point.  This leaves one to
// wonder: when bits are duplicated, one can be left playing the game: "guess
// the bit which was used" when calling Chmod?  Besides, what can go wrong when
// bits at two locations in a register could trigger SUID on a root owned
// executable?
//
// Ref: https://cs.opensource.google/go/go/+/master:src/os/file_posix.go;l=62-75
func FileModePerm(m fs.FileMode) Mode {
	return Mode(m&0777 | m&fs.ModeSetuid>>12 | m&fs.ModeSetgid>>12 | m&fs.ModeSticky>>11)
}

// Create a unixmode.Mode(uint16) from a FileMode(uint32) by cross referencing bits.
func New(m fs.FileMode) Mode {
	out := Mode(m&0777 | m&fs.ModeSetuid>>12 | m&fs.ModeSetgid>>12 | m&fs.ModeSticky>>11)

	switch m & fs.ModeType {
	/* These are the most common.  */
	case 0:
		out |= ModeRegular
	case fs.ModeDir:
		out |= ModeDir

	/* Other letters standardized by POSIX 1003.1-2004.  */
	case fs.ModeDevice:
		out |= ModeDevice
	case fs.ModeCharDevice | fs.ModeDevice:
		out |= ModeCharDevice
	case fs.ModeSymlink:
		out |= ModeSymlink
	case fs.ModeNamedPipe:
		out |= ModeNamedPipe

	/* Other file types (though not letters) standardized by POSIX.  */
	case fs.ModeSocket:
		out |= ModeSocket
	}
	return out
}

// Create a FileMode(uint32) from a unixmode.Mode(uint16) by cross referencing bits.
func (m Mode) FileMode() fs.FileMode {
	out := fs.FileMode(m)&0777 | fs.FileMode(m)&06000<<12 | fs.FileMode(m)&01000<<11
	switch m & ModeTypeMask {
	/* These are the most common.  */
	case ModeRegular:
		// Do nothing
	case ModeDir:
		out |= fs.ModeDir

	/* Other letters standardized by POSIX 1003.1-2004.  */
	case ModeDevice:
		out |= fs.ModeDevice
	case ModeCharDevice:
		out |= fs.ModeCharDevice | fs.ModeDevice
	case ModeSymlink:
		out |= fs.ModeSymlink
	case ModeNamedPipe:
		out |= fs.ModeNamedPipe

	/* Other file types (though not letters) standardized by POSIX.  */
	case ModeSocket:
		out |= fs.ModeSocket
	}
	return out
}

// Type returns type bits in m (m & ModeTypeMask).
func (m Mode) Type() Mode {
	return m & ModeTypeMask
}

// Functional call to os.Chmod with the ability to set permissions based on the
// Mode value.
func Chmod(name string, m Mode) error {
	// Move the bits back up to the higher order values and then call Chmod
	return os.Chmod(name, fs.FileMode(m)&0777|fs.FileMode(m)&06000<<12|fs.FileMode(m)&01000<<11)
}
