package main

import (
	"fmt"
	"time"

	"github.com/jensenak/emu16/emu"
)

// Mem is system memory
type Mem struct {
	bank [16384]uint8
}

// Load8 return a byte
func (m *Mem) Load8(addr, offset uint16) (uint8, error) {
	return 0, nil
}

// Load16 returns 2 bytes
func (m *Mem) Load16(addr, offset uint16) (uint16, error) {
	return 0, nil
}

// Save8 stores a byte
func (m *Mem) Save8(addr, offset uint16, data uint8) error {
	return nil
}

// Save16 stores 2 bytes
func (m *Mem) Save16(addr, offset, data uint16) error {
	return nil
}

// Bus is for communication
type Bus struct {
	name string
	C    <-chan emu.Interrupt
}

// Send is to put data on a bus
func (b *Bus) Send(addr, data uint8) error {
	return nil
}

// Recv gets data off a bus
func (b *Bus) Recv(addr uint8) (uint8, error) {
	return 0, nil
}

// Interrupts receives an interrupt chan from cpu
func (b *Bus) Interrupts(c <-chan emu.Interrupt) {
	b.C = c
	return
}

// Bootmedia initializes memory and cpu
type Bootmedia struct {
	name string
}

// GetOffset tells us where to start writing mem
func (b *Bootmedia) GetOffset() (uint16, error) {
	return 0, nil
}

// GetLength states how much data is to be loaded from bootmedia
func (b *Bootmedia) GetLength() (uint16, error) {
	return 0, nil
}

// GetIP returns the initial instruction pointer
func (b *Bootmedia) GetIP() (uint16, error) {
	return 0, nil
}

// Load gets the boot data at a certain byte
func (b *Bootmedia) Load(addr uint16) (uint8, error) {
	return 0, nil
}

func main() {
	fmt.Println("Starting")
	tick := time.NewTicker(time.Millisecond * 200).C
	m := Mem{}
	bm := Bootmedia{}
	bu := Bus{}
	proc := emu.NewProcessor(&m, &bm, &bu, tick)
	proc.Boot()
	proc.Run()
}
