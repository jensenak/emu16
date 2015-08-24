package emu

import (
	"errors"
	"fmt"
	"time"
)

// These constants are our instructions
const (
	LOAD = iota
	STORE
	SETL
	SETH
	SBUS
	RBUS
	LJUMP
	EJUMP
	ADD
	SUB
	SHL
	SHR
	AND
	OR
	NOT
	XOR
)

// Interrupt is used to force the processor to run an alternate code segment
type Interrupt struct {
	BusAddr uint8  // Which bus sent the interrupt
	Handler uint16 // What address contains the code to handle event
}

// Register represents CPU internal storage
type Register struct {
	High uint8
	Low  uint8
}

// Put16 saves a uint16 in Big Endian to the register
func (r *Register) Put16(d uint16) {
	r.High = uint8(d >> 8)
	r.Low = uint8(d & 0xFF)
}

// Get16 converts the two bytes in the reg to a uint16
func (r *Register) Get16() uint16 {
	return uint16(r.High)<<8 | uint16(r.Low)
}

// Processor represents the core of this whole machine! :D
type Processor struct {
	Register [16]Register
	Memory
	Bootmedia
	Bus
	Ticker <-chan time.Time
	Ints   <-chan Interrupt
}

// ProcError used to return errors
type ProcError struct {
	msg    string
	code   int
	addr   uint16
	offset uint16
	data   []uint8
	orig   error
}

func (pe ProcError) Error() string {
	out := fmt.Sprintf("%s [%d] | [addr] %x [offset] %x\n[data] %x", pe.msg, pe.code, pe.addr, pe.offset, pe.data)
	if pe.orig != nil {
		out += fmt.Sprintf("-- ORIGINAL ERROR --\n%s\n-- END --", pe.orig)
	}
	return out
}

// Memory loads and saves data
type Memory interface {
	Load8(address, offset uint16) (uint8, error)
	Save8(address, offset uint16, data uint8) error
	Load16(address, offset uint16) (uint16, error)
	Save16(address, offset, data uint16) error
}

// Bootmedia is the initial source of instructions
type Bootmedia interface {
	GetOffset() (uint16, error)
	GetLength() (uint16, error)
	Load(offset uint16) (uint8, error)
	GetIP() (uint16, error)
}

// Bus is a general purpose interface for interacting with the processor
// busses 0 - 5 planned for normal use
// bus 15 reserved for signalling
type Bus interface {
	Send(busaddr, data uint8) error
	Recv(busaddr uint8) (uint8, error)
	Interrupts(<-chan Interrupt)
}

// NewProcessor - Basically just filling the struct for you.
func NewProcessor(m Memory, boot Bootmedia, bus Bus, t <-chan time.Time) Processor {
	regs := [16]Register{}
	ints := make(chan Interrupt)
	bus.Interrupts(ints) // Give all busses our interrupt chan
	return Processor{regs, m, boot, bus, t, ints}
}

// Boot loads data from Bootmedia
func (p *Processor) Boot() error {
	offset, err := p.Bootmedia.GetOffset()
	if err != nil {
		return errors.New("Failed to load offset from bootmedia")
	}
	length, err := p.Bootmedia.GetLength()
	if err != nil {
		return errors.New("Failed to load length from bootmedia")
	}
	for addr := uint16(0); addr < length; addr++ {
		data, err := p.Bootmedia.Load(addr)
		if err != nil {
			return ProcError{"Failed to load data from bootmedia", 0, addr, offset, nil, err}
		}
		err = p.Memory.Save8(addr, offset, data)
		if err != nil {
			return ProcError{"Failed to save data to memory", 0, addr, offset, []uint8{data}, err}
		}
	}
	ip, err := p.Bootmedia.GetIP() // Get initial instruction pointer
	if err != nil {
		return fmt.Errorf("Could not set initial Instruction Pointer: " + err.Error())
	}
	p.Register[15].Put16(ip)
	return nil
}

// Run does what you'd expect
func (p *Processor) Run() error {
Mainloop:
	for {
		err := p.execute()
		if err != nil {
			return err
		}
		select {
		case <-p.Ticker:
		case i := <-p.Ints:
			p.Register[15].Put16(i.Handler)
			break Mainloop
		}
		p.Register[15].Put16(p.Register[15].Get16() + 1)
	}
	return nil
}

func (p *Processor) execute() error {
	var data uint16
	inst, err := p.Memory.Load16(p.Register[15].Get16(), 0)
	if err != nil {
		return err
	}
	opcode := uint8(inst >> 12)
	arg1 := uint8(inst & 0xF00 >> 8)
	arg2 := uint8(inst & 0xF0 >> 4)
	arg3 := uint8(inst & 0xF)
	switch opcode {
	case LOAD:
		if arg3 > 0 {
			p.Register[arg1].Low, err = p.Memory.Load8(p.Register[arg2].Get16(), p.Register[arg3].Get16())
		} else {
			data, err = p.Memory.Load16(p.Register[arg2].Get16(), 0)
			p.Register[arg1].Put16(data)
		}
	case STORE:
		if arg3 > 0 {
			err = p.Memory.Save8(p.Register[arg2].Get16(), p.Register[arg3].Get16(), p.Register[arg1].Low)
		} else {
			err = p.Memory.Save16(p.Register[arg2].Get16(), 0, p.Register[arg1].Get16())
		}
	case SETL:
		p.Register[arg1].Low = arg2<<4 | arg3
	case SETH:
		p.Register[arg1].High = arg2<<4 | arg3
	case SBUS:
		err = p.Bus.Send(arg2, arg1)
	case RBUS:
		p.Register[arg1].Low, err = p.Bus.Recv(arg2)
	case LJUMP:
		if p.Register[arg1].Get16() < p.Register[arg2].Get16() {
			p.Register[15] = p.Register[arg3]
		}
	case EJUMP:
		if p.Register[arg1].Get16() == p.Register[arg2].Get16() {
			p.Register[15] = p.Register[arg3]
		}
	case ADD:
		data = p.Register[arg2].Get16() + p.Register[arg3].Get16()
		p.Register[arg1].Put16(data)
	case SUB:
		data = p.Register[arg2].Get16() - p.Register[arg3].Get16()
		p.Register[arg1].Put16(data)
	case SHL:
		data = p.Register[arg2].Get16() << p.Register[arg3].Get16()
		p.Register[arg1].Put16(data)
	case SHR:
		data = p.Register[arg2].Get16() >> p.Register[arg3].Get16()
		p.Register[arg1].Put16(data)
	case AND:
		data = p.Register[arg2].Get16() & p.Register[arg3].Get16()
		p.Register[arg1].Put16(data)
	case OR:
		data = p.Register[arg2].Get16() | p.Register[arg3].Get16()
		p.Register[arg1].Put16(data)
	case NOT:
		data = p.Register[arg2].Get16() ^ uint16(0xFFFF)
		p.Register[arg1].Put16(data)
	case XOR:
		data = p.Register[arg2].Get16() ^ p.Register[arg3].Get16()
		p.Register[arg1].Put16(data)
	default:
		return nil
	}
	// Since each case performs one op, we can catch all errors here.
	if err != nil {
		return err
	}
	return nil
}
