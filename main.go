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
		return 0, fmt.Errorf("Segfault (accessing 8 %x + offset %x)", addr, offset)
	}
	return m.bank[addr+offset], nil
}

// Load16 returns 2 bytes
func (m *Mem) Load16(addr, offset uint16) (uint16, error) {
	if addr+offset+1 > m.bankSize {
		return 0, fmt.Errorf("Segfault (accessing 16 %x + offset %x)", addr, offset)
	}
	return uint16(m.bank[addr+offset])<<8 | uint16(m.bank[addr+offset+1]), nil
}

// Save8 stores a byte
func (m *Mem) Save8(addr, offset uint16, data uint8) error {
	if addr+offset > m.bankSize {
		return fmt.Errorf("Segfault (saving 8 %x + offset %x)", addr, offset)
	}
	m.bank[addr+offset] = data
	return nil
}

// Save16 stores 2 bytes
func (m *Mem) Save16(addr, offset, data uint16) error {
	if addr+offset+1 > m.bankSize {
		return fmt.Errorf("Segfault (saving 16 %x + offset %x)", addr, offset)
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
	c    chan<- emu.Interrupt
	ch   []channels
	wait []uint8
}

type channels struct {
	out chan uint16 // output <- cpu
	in  chan uint16 // data -> cpu
}

func (b *Bus) newBus(buffer int) int {
	in := make(chan uint16, buffer)
	out := make(chan uint16, buffer)
	chans := channels{out, in}
	b.ch = append(b.ch, chans)
	return len(b.ch) - 1
}

// Send is to put data on a bus
func (b *Bus) Send(addr uint8, data uint16) error {
	if uint8(len(b.ch)) < addr {
		return errors.New("Invalid bus address")
	}
	b.ch[addr].out <- data
	return nil
}

// Recv gets data off a bus
func (b *Bus) Recv(addr uint8) (uint16, error) {
	if uint8(len(b.ch)) < addr {
		return 0, errors.New("Invalid bus address")
	}
	return <-b.ch[addr].in, nil
}

// Which returns the address of the first bus with waiting data
func (b *Bus) Which() (uint8, error) {
	if len(b.wait) > 0 {
		return b.wait[0], nil
	}
	return 0, errors.New("No data")
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

	// For the following program, registers are used as follows
	// 15 - Instruction pointer (reserved)
	// 0 - value
	// 1 - multiplier
	// 2 - result
	// 5 - bus driver
	// 10 - zero
	// 11 - one
	// 12 - jump addr
	data := []uint8{
		0x0a, 0x0f, // Just some vars (`10` & `15`)
		// Silently leaving reg a at 0
		0x2b, 0x00, 0x01, // Set reg b to `1`
		0x00, 0xa1, // load var into 0
		0x01, 0xb1, // load second var into 1
		0x2c, 0x00, 0x12, //8 Set the jump location
		0x60, 0x1c, // compare: if 0 < 1 jump
		0x01, 0xa0, // Swap vars (so smallest is in 0)
		0x00, 0xb0, //
		0x82, 0x21, // Add value to our result, store in result
		0x90, 0x0b, // Sub 11 (one) from 0
		0x25, 0x00, 0x02, // Prep bus driver to deliver result to tty
		0x45,       // Send the result over bus
		0x6a, 0x0c, // if 1 is still larger than 10 jump back
		0x25, 0x01, 0x0b, // Prep bus driver to kill process
		0x45, // And quit
	}
	// Data from above, load into beginning of memory (0), and start instruction pointer at 0x02
	e := bm.init(data, 0, 2)
	if e != nil {
		panic(e)
	}

	bu := Bus{}
	tty := bu.newBus(2)
	done := bu.newBus(2)

	fmt.Printf("done\nCreating new processor...")
	proc := emu.NewProcessor(&m, &bm, &bu, tick)
	fmt.Printf("done\nBooting...")
	proc.Boot()
	fmt.Printf("done\nRunning processor\n\n")

	errorChan := make(chan error)
	go proc.Run(errorChan)

	tick2 := time.NewTicker(time.Millisecond * 100).C
Mainloop:
	for {
		//terminal(0, 10, &m)
		select {
		case e := <-errorChan:
			fmt.Printf("\n-- Error: %s --\n", e)
			for i := 0; i < 40; i++ {
				fmt.Printf("0x%x 0x%x \n", i, m.bank[i])
			}
			break Mainloop
		case output := <-bu.ch[tty].out:
			fmt.Printf("%d ", output)
		case <-bu.ch[done].out:
			fmt.Println("\nDone")
			close(bu.c)
			break Mainloop
		case <-tick2:
		}
	}
}

func terminal(start, end int, m *Mem) {
	for i := start; i <= end; i++ {
		fmt.Printf("\033[%d;%dH", 5, i*8)
		fmt.Printf("%x:%x", i, m.bank[i])
	}
}
