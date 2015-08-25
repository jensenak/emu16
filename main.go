package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/jensenak/emu16/emu"
)

//==================================================\\
// MEMORY Modules
//==================================================\\

// Mem is system memory
type Mem struct {
	bank     []uint8
	bankSize uint16
}

func (m *Mem) newBanks(length uint16) {
	m.bank = make([]uint8, length)
	m.bankSize = length
}

// Load8 return a byte
func (m *Mem) Load8(addr, offset uint16) (uint8, error) {
	if addr+offset > m.bankSize {
		return 0, errors.New("Segfault!")
	}
	return m.bank[addr+offset], nil
}

// Load16 returns 2 bytes
func (m *Mem) Load16(addr, offset uint16) (uint16, error) {
	if addr+offset+1 > m.bankSize {
		return 0, errors.New("Segfault!")
	}
	return uint16(m.bank[addr+offset])<<8 | uint16(m.bank[addr+offset+1]), nil
}

// Save8 stores a byte
func (m *Mem) Save8(addr, offset uint16, data uint8) error {
	if addr+offset > m.bankSize {
		return errors.New("Segfault!")
	}
	m.bank[addr+offset] = data
	return nil
}

// Save16 stores 2 bytes
func (m *Mem) Save16(addr, offset, data uint16) error {
	if addr+offset+1 > m.bankSize {
		return errors.New("Segfault!")
	}
	m.bank[addr+offset] = uint8(data >> 8)
	m.bank[addr+offset+1] = uint8(data & 0xFF)
	return nil
}

//==================================================\\
// BUS Modules
//==================================================\\

// Bus is for communication
type Bus struct {
	c  chan<- emu.Interrupt
	ch []channels
}

type channels struct {
	out chan uint8 // output <- cpu
	in  chan uint8 // data -> cpu
}

func (b *Bus) newBus(buffer int) uint8 {
	in := make(chan uint8, buffer)
	out := make(chan uint8, buffer)
	chans := channels{out, in}
	b.ch = append(b.ch, chans)
	return uint8(len(b.ch))
}

// Send is to put data on a bus
func (b *Bus) Send(addr, data uint8) error {
	if uint8(len(b.ch)) < addr {
		return errors.New("Invalid bus address")
	}
	b.ch[addr].out <- data
	return nil
}

// Recv gets data off a bus
func (b *Bus) Recv(addr uint8) (uint8, error) {
	if uint8(len(b.ch)) < addr {
		return 0, errors.New("Invalid bus address")
	}
	return <-b.ch[addr].in, nil
}

// Interrupts receives an interrupt chan from cpu
func (b *Bus) Interrupts(c chan<- emu.Interrupt) {
	b.c = c
	return
}

//==================================================\\
// BOOTMEDIA Modules
//==================================================\\

// Bootmedia initializes memory and cpu
type Bootmedia struct {
	offset uint16
	length uint16
	start  uint16
	data   []uint8
}

func (b *Bootmedia) init(data []uint8, offset, start uint16) error {
	b.data = data
	b.length = uint16(len(data))
	b.offset = offset
	b.start = start
	return nil
}

// GetOffset tells us where to start writing mem
func (b *Bootmedia) GetOffset() (uint16, error) {
	return b.offset, nil
}

// GetLength states how much data is to be loaded from bootmedia
func (b *Bootmedia) GetLength() (uint16, error) {
	return b.length, nil
}

// GetIP returns the initial instruction pointer
func (b *Bootmedia) GetIP() (uint16, error) {
	return b.start, nil
}

// Load gets the boot data at a certain byte
func (b *Bootmedia) Load(addr uint16) (uint8, error) {
	if addr > b.length {
		return 0, errors.New("Load outside of bootmedia")
	}
	return b.data[addr], nil
}

//===============================
// MAIN BODY
//===============================

func main() {
	fmt.Printf("Initializing resources...")
	tick := time.NewTicker(time.Millisecond * 200).C
	m := Mem{}
	m.newBanks(16384) // Init with 16K of ram

	bm := Bootmedia{}
	data := []uint8{}
	e := bm.init(data, 0, 0)
	if e != nil {
		panic(e)
	}

	bu := Bus{}
	tty := bu.newBus(0)
	done := bu.newBus(0)

	fmt.Printf("done\nCreating new processor...")
	proc := emu.NewProcessor(&m, &bm, &bu, tick)
	fmt.Printf("done\nBooting...")
	proc.Boot()
	fmt.Printf("done\nRunning processor.")
	go proc.Run()
	for {
		select {
		case output := <-bu.ch[tty].out:
			fmt.Printf("%d", output)
		case <-bu.ch[done].out:
			fmt.Println("DONE")
			close(bu.c)
		}
	}
}
