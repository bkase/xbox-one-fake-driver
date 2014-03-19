// Copyright 2013 Kyle Lemons.  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"time"

	"github.com/kylelemons/gousb/usb"
)

var (
	readonly = flag.Bool("readonly", false, "Only read from the controller")
	debug    = flag.Int("debug", 0, "USB debugging control")
)

type modelInfo struct {
	config, iface, setup, endIn, endOut uint8
	kind                                string
}

func connectToSocket() net.Conn {
	conn, err := net.Dial("unixgram", "/tmp/keys.sock")
	if err != nil {
		log.Panic("Did you forget to start the python server?")
		panic(err)
	}
	return conn
}

func main() {
	flag.Parse()

	ctx := usb.NewContext()
	defer ctx.Close()

	if *debug != 0 {
		ctx.Debug(*debug)
	}

	var model modelInfo

	devs, err := ctx.ListDevices(func(desc *usb.Descriptor) bool {
		switch {
		case desc.Vendor == 0x045e && desc.Product == 0x028e:
			log.Printf("Found standard Microsoft controller")
			/*
			   250.006 045e:028e Xbox360 Controller (Microsoft Corp.)
			     Protocol: Vendor Specific Class (Vendor Specific Subclass) Vendor Specific Protocol
			     Config 01:
			       --------------
			       Interface 00 Setup 00
			         Vendor Specific Class
			         Endpoint 1 IN  interrupt - unsynchronized data [32 0]
			         Endpoint 1 OUT interrupt - unsynchronized data [32 0]
			       --------------
			       Interface 01 Setup 00
			         Vendor Specific Class
			         Endpoint 2 IN  interrupt - unsynchronized data [32 0]
			         Endpoint 2 OUT interrupt - unsynchronized data [32 0]
			         Endpoint 3 IN  interrupt - unsynchronized data [32 0]
			         Endpoint 3 OUT interrupt - unsynchronized data [32 0]
			       --------------
			       Interface 02 Setup 00
			         Vendor Specific Class
			         Endpoint 0 IN  interrupt - unsynchronized data [32 0]
			       --------------
			       Interface 03 Setup 00
			         Vendor Specific Class
			       --------------
			*/
			model = modelInfo{1, 0, 0, 1, 1, "360"}

		case desc.Vendor == 0x045e && desc.Product == 0x02d1:
			log.Printf("Found Microsoft Xbox One controller")
			/*
			   250.006 045e:02d1 Unknown (Microsoft Corp.)
			     Protocol: Vendor Specific Class
			     Config 01:
			       --------------
			       Interface 00 Setup 00
			         Vendor Specific Class
			         Endpoint 1 OUT interrupt - unsynchronized data [64 0]
			         Endpoint 1 IN  interrupt - unsynchronized data [64 0]
			       --------------
			       Interface 01 Setup 00
			         Vendor Specific Class
			       Interface 01 Setup 01
			         Vendor Specific Class
			         Endpoint 2 OUT isochronous - unsynchronized data [228 0]
			         Endpoint 2 IN  isochronous - unsynchronized data [228 0]
			       --------------
			       Interface 02 Setup 00
			         Vendor Specific Class
			       Interface 02 Setup 01
			         Vendor Specific Class
			         Endpoint 3 OUT bulk - unsynchronized data [64 0]
			         Endpoint 3 IN  bulk - unsynchronized data [64 0]
			       --------------
			*/
			model = modelInfo{1, 0, 0, 1, 1, "one"}

		case desc.Vendor == 0x1689 && desc.Product == 0xfd00:
			log.Printf("Found Razer Onza Tournament controller")
			/*
				250.006 1689:fd00 Unknown 1689:fd00
				  Protocol: Vendor Specific Class (Vendor Specific Subclass) Vendor Specific Protocol
				  Config 01:
				    --------------
				    Interface 00 Setup 00
				      Vendor Specific Class
				      Endpoint 1 IN  interrupt - unsynchronized data [32 0]
				      Endpoint 2 OUT interrupt - unsynchronized data [32 0]
				    --------------
				    Interface 01 Setup 00
				      Vendor Specific Class
				      Endpoint 3 IN  interrupt - unsynchronized data [32 0]
				      Endpoint 0 OUT interrupt - unsynchronized data [32 0]
				      Endpoint 1 IN  interrupt - unsynchronized data [32 0]
				      Endpoint 1 OUT interrupt - unsynchronized data [32 0]
				    --------------
				    Interface 02 Setup 00
				      Vendor Specific Class
				      Endpoint 2 IN  interrupt - unsynchronized data [32 0]
				    --------------
				    Interface 03 Setup 00
				      Vendor Specific Class
				    --------------
			*/
			model = modelInfo{1, 0, 0, 1, 2, "360"}
		default:
			return false
		}
		return true
	})
	if err != nil {
		log.Fatalf("listdevices: %s", err)
	}
	defer func() {
		for _, d := range devs {
			d.Close()
		}
	}()
	if len(devs) != 1 {
		log.Fatalf("found %d devices, want 1", len(devs))
	}
	controller := devs[0]

	if err := controller.Reset(); err != nil {
		log.Fatalf("reset: %s", err)
	}

	in, err := controller.OpenEndpoint(
		model.config,
		model.iface,
		model.setup,
		model.endIn|uint8(usb.ENDPOINT_DIR_IN))
	if err != nil {
		log.Fatalf("in: openendpoint: %s", err)
	}

	out, err := controller.OpenEndpoint(
		model.config,
		model.iface,
		model.setup,
		model.endOut|uint8(usb.ENDPOINT_DIR_OUT))
	if err != nil {
		log.Fatalf("out: openendpoint: %s", err)
	}

	switch {
	case *readonly:
		var b [512]byte
		for {
			n, err := in.Read(b[:])
			log.Printf("read %d bytes: % x [err: %v]", n, b[:n], err)
			if err != nil {
				break
			}
		}
	case model.kind == "360":
		XBox360(controller, in, out)
	case model.kind == "one":
		XBoxOne(controller, in, out)
	}
}

func XBox360(controller *usb.Device, in, out usb.Endpoint) {
	// https://github.com/Grumbel/xboxdrv/blob/master/PROTOCOL

	const (
		Empty      byte = iota // 00000000 ( 0) no LEDs
		WarnAll                // 00000001 ( 1) flash all briefly
		NewPlayer1             // 00000010 ( 2) p1 flash then solid
		NewPlayer2             // 00000011
		NewPlayer3             // 00000100
		NewPlayer4             // 00000101
		Player1                // 00000110 ( 6) p1 solid
		Player2                // 00000111
		Player3                // 00001000
		Player4                // 00001001
		Waiting                // 00001010 (10) empty w/ loops
		WarnPlayer             // 00001011 (11) flash active
		_                      // 00001100 (12) empty
		Battery                // 00001101 (13) squiggle
		Searching              // 00001110 (14) slow flash
		Booting                // 00001111 (15) solid then flash
	)

	led := func(b byte) {
		out.Write([]byte{0x01, 0x03, b})
	}

	setPlayer := func(player byte) {
		spin := []byte{
			Player1, Player2, Player4, Player3,
		}
		spinIdx := 0
		spinDelay := 100 * time.Millisecond

		led(Booting)
		time.Sleep(100 * time.Millisecond)
		for spinDelay > 20*time.Millisecond {
			led(spin[spinIdx])
			time.Sleep(spinDelay)
			spinIdx = (spinIdx + 1) % len(spin)
			spinDelay -= 5 * time.Millisecond
		}
		for i := 0; i < 40; i++ { // just for safety
			cur := spin[spinIdx]
			led(cur)
			time.Sleep(spinDelay)
			spinIdx = (spinIdx + 1) % len(spin)
			if cur == player {
				break
			}
		}
	}

	led(Empty)
	time.Sleep(1 * time.Second)
	setPlayer(Player1)

	var b [512]byte
	for {
		n, err := in.Read(b[:])
		log.Printf("read %d bytes: % x [err: %v]", n, b[:n], err)
		if err != nil {
			break
		}
	}

	/*
		time.Sleep(1 * time.Second)
		setPlayer(Player2)
		time.Sleep(1 * time.Second)
		setPlayer(Player3)
		time.Sleep(1 * time.Second)
		setPlayer(Player4)
		time.Sleep(5 * time.Second)
		led(Waiting)
	*/

	var last, cur [512]byte
	decode := func() {
		n, err := in.Read(cur[:])
		if err != nil || n != 20 {
			log.Printf("ignoring read: %d bytes, err = %v", n, err)
			return
		}

		// 1-bit values
		for _, v := range []struct {
			idx  int
			bit  uint
			name string
		}{
			{2, 0, "DPAD U"},
			{2, 1, "DPAD D"},
			{2, 2, "DPAD L"},
			{2, 3, "DPAD R"},
			{2, 4, "START"},
			{2, 5, "BACK"},
			{2, 6, "THUMB L"},
			{2, 7, "THUMB R"},
			{3, 0, "LB"},
			{3, 1, "RB"},
			{3, 2, "GUIDE"},
			{3, 4, "A"},
			{3, 5, "B"},
			{3, 6, "X"},
			{3, 7, "Y"},
		} {
			c := cur[v.idx] & (1 << v.bit)
			l := last[v.idx] & (1 << v.bit)
			if c == l {
				continue
			}
			switch {
			case c != 0:
				log.Printf("Button %q pressed", v.name)
			case l != 0:
				log.Printf("Button %q released", v.name)
			}
		}

		// 8-bit values
		for _, v := range []struct {
			idx  int
			name string
		}{
			{4, "LT"},
			{5, "RT"},
		} {
			c := cur[v.idx]
			l := last[v.idx]
			if c == l {
				continue
			}
			log.Printf("Trigger %q = %v", v.name, c)
		}

		dword := func(hi, lo byte) int16 {
			return int16(hi)<<8 | int16(lo)
		}

		//     +y
		//      N
		// -x W-|-E +x
		//      S
		//     -y
		dirs := [...]string{
			"W", "SW", "S", "SE", "E", "NE", "N", "NW", "W",
		}
		dir := func(x, y int16) (string, int32) {
			// Direction
			rad := math.Atan2(float64(y), float64(x))
			dir := 4 * rad / math.Pi
			card := int(dir + math.Copysign(0.5, dir))

			// Magnitude
			mag := math.Sqrt(float64(x)*float64(x) + float64(y)*float64(y))
			return dirs[card+4], int32(mag)
		}

		// 16-bit values
		for _, v := range []struct {
			hiX, loX int
			hiY, loY int
			name     string
		}{
			{7, 6, 9, 8, "LS"},
			{11, 10, 13, 12, "RS"},
		} {
			c, cmag := dir(
				dword(cur[v.hiX], cur[v.loX]),
				dword(cur[v.hiY], cur[v.loY]),
			)
			l, lmag := dir(
				dword(last[v.hiX], last[v.loX]),
				dword(last[v.hiY], last[v.loY]),
			)
			ccenter := cmag < 10240
			lcenter := lmag < 10240
			if ccenter && lcenter {
				continue
			}
			if c == l && cmag == lmag {
				continue
			}
			if cmag > 10240 {
				log.Printf("Stick %q = %v x %v", v.name, c, cmag)
			} else {
				log.Printf("Stick %q centered", v.name)
			}
		}

		last, cur = cur, last
	}

	controller.ReadTimeout = 60 * time.Second
	for {
		decode()
	}
}

func XBoxOne(controller *usb.Device, in, out usb.Endpoint) {

	conn := connectToSocket()

	var b [64]byte
	read := func() (tag, code byte, data []byte, err error) {
		n, err := in.Read(b[:])
		log.Printf("read %d bytes: % x [err: %v]", n, b[:n], err)

		if err != nil {
			return 0, 0, nil, err
		}
		if n < 2 {
			return 0, 0, nil, fmt.Errorf("only read %d bytes", n)
		}

		return b[0], b[1], b[2:n], nil
	}

	write := func(data ...byte) error {
		n, err := out.Write(data)
		log.Printf("sent %d bytes: % x [err: %v]", n, data, err)
		if n < len(data) {
			return fmt.Errorf("only sent %d of %d bytes", n, len(data))
		}
		return err
	}

	dieIf := func(err error, format string, args ...interface{}) {
		if err == nil {
			return
		}
		msg := fmt.Sprintf(format, args...)
		log.Fatalf("%s: %s", msg, err)
	}

	var err error

	// Initializ
	err = write(0x05, 0x20)
	dieIf(err, "initialization")

	decode := func(data []byte, last []byte) {
		if len(data) != 16 {
			log.Printf("Only got %d bytes (want 16)", len(data))
		}
		var (
			_    = data[0] // sequence number
			_    = data[1] // unknown
			btn1 = data[2] // yxbaSM?N S=share, M=Menu, N=Sync
			_    = btn1
			btn2 = data[3] // rlrlRLDU r=R-Stick/Trigger, l=L-Stick/Trigger, R=D-Right, L=D-Left, D=D-Down, U=D-Up
			_    = btn2

			lt = binary.LittleEndian.Uint16(data[4:6]) // left trigger, 0..1024
			rt = binary.LittleEndian.Uint16(data[6:8]) // right trigger

			lx = int16(binary.LittleEndian.Uint16(data[8:10]))  // left stick X, +/- 32768 or so
			ly = int16(binary.LittleEndian.Uint16(data[10:12])) // left stick Y
			rx = int16(binary.LittleEndian.Uint16(data[12:14])) // right stick X
			ry = int16(binary.LittleEndian.Uint16(data[14:16])) // right stick Y
		)

		// btn1, least to most significant
		for _, btn := range []struct {
			idx  int
			bit  uint
			name string
			key  string
		}{
			{2, 0, "SYNC", "8"},
			{2, 1, "BTN1|0x02", "7"},
			{2, 2, "MENU", "6"},
			{2, 3, "SHARE", "5"},
			{2, 4, "A", " "},
			{2, 5, "B", "v"},
			{2, 6, "X", "z"},
			{2, 7, "Y", "x"},
			{3, 0, "D-Up", "a"},
			{3, 1, "D-Down", "q"},
			{3, 2, "D-Left", "1"},
			{3, 3, "D-Right", "2"},
			{3, 4, "L-Trigger", "g"},
			{3, 5, "R-Trigger", "c"},
			{3, 6, "L-Stick", "3"},
			{3, 7, "R-Stick", "4"},
		} {
			c := data[btn.idx] & (1 << btn.bit)
			l := last[btn.idx] & (1 << btn.bit)
			if c == l {
				continue
			}
			switch {
			case c != 0:
				log.Printf("Button %q pressed: %q", btn.name, btn.key)
				conn.Write([]byte("d" + btn.key))
			case l != 0:
				log.Printf("Button %q released: %q", btn.name, btn.key)
				conn.Write([]byte("u" + btn.key))
			}
		}

		if lt > 0 || rt > 0 {
			log.Println("LT:", lt, "RT:", rt)
		}

		data[8] = 0

		check_for_press := func(isOne bool, idx int, bit uint) int {
			var val int
			if isOne {
				val = 1
			} else {
				val = 0
			}

			data[idx] |= byte(val << uint(bit))
			l := last[idx] & (1 << uint(bit))
			c := data[idx] & (1 << uint(bit))
			if c == l {
				return 0
			}
			switch {
			case c != 0:
				return 1
			case l != 0:
				return -1
			}
			return 0
		}

		// threshold the joystick
		for _, stick := range []struct {
			idx      int
			bit      uint
			value    int16
			neighbor int16
			name     string
			keyPos   string
			keyNeg   string
		}{
			{8, 0, lx, ly, "L_X-AXIS", "Right", "Left"},
			{8, 2, ly, lx, "L_Y-AXIS", "Up", "Down"},
			{8, 4, rx, ry, "R_X-AXIS", "1", "2"},
			{8, 6, ry, rx, "R_Y-AXIS", "3", "4"},
		} {
			pos := check_for_press((stick.value > 16384), stick.idx, stick.bit)
			if pos > 0 {
				log.Printf("Button %q pressed: %q", "+"+stick.name, stick.keyPos)
				conn.Write([]byte("d" + stick.keyPos))
			} else if pos < 0 {
				log.Printf("Button %q released: %q", "+"+stick.name, stick.keyPos)
				conn.Write([]byte("u" + stick.keyPos))
			}

			neg := check_for_press((stick.value < -16384), stick.idx, stick.bit+1)
			if neg > 0 {
				log.Printf("Button %q pressed: %q", "-"+stick.name, stick.keyNeg)
				conn.Write([]byte("d" + stick.keyNeg))
			} else if neg < 0 {
				log.Printf("Button %q released: %q", "-"+stick.name, stick.keyNeg)
				conn.Write([]byte("u" + stick.keyNeg))
			}
		}
	}

	last := make([]byte, 62)
	for {
		tag, _, data, _ := read()
		switch tag {
		case 0x07:
			if len(data) != 4 {
				log.Printf("Wanted %d bytes, got %d", 4, len(data))
				break
			}
			if data[2]&0x01 != 0 {
				log.Print("GUIDE")
			}
		case 0x20:
			decode(data, last)
			copy(last[:], data[:])
		}
	}
}
