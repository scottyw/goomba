package gb

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/scottyw/tetromino/pkg/gb/cpu"
	"github.com/scottyw/tetromino/pkg/gb/lcd"
	"github.com/scottyw/tetromino/pkg/gb/mem"
	"github.com/scottyw/tetromino/pkg/gb/timer"
	"github.com/scottyw/tetromino/pkg/ui"
)

const frameDuration = float64(16742706)

type gui interface {
	DrawFrame(image *image.RGBA)
}

// Options control emulator behaviour
type Options struct {
	RomFilename string
	Fast        bool
	DebugCPU    bool
	DebugTimer  bool
	DebugLCD    bool
	SBWriter    io.Writer
}

// Gameboy represents the Gameboy itself
type Gameboy struct {
	dispatch *cpu.Dispatch
	memory   *mem.Memory
	timer    *timer.Timer
	lcd      *lcd.LCD
	start    time.Time
	opts     Options
	cancel   func()
	dur      time.Duration
	frame    int
}

// NewGameboy returns a new Gameboy
func NewGameboy(opts Options, cancel func()) Gameboy {
	var rom []byte
	if opts.RomFilename == "" {
		rom = make([]byte, 0x8000)
	} else {
		rom = readRomFile(opts.RomFilename)
	}
	c := cpu.NewCPU(opts.DebugCPU)
	timer := timer.NewTimer(opts.DebugTimer)
	memory := mem.NewMemory(rom, opts.SBWriter, timer)
	dispatch := cpu.NewDispatch(c, memory)
	lcd := lcd.NewLCD(memory, opts.DebugLCD)
	start := time.Now()
	duration := frameDuration
	if opts.Fast {
		duration = 0
	}
	return Gameboy{
		dispatch: dispatch,
		memory:   memory,
		timer:    timer,
		lcd:      lcd,
		start:    start,
		opts:     opts,
		cancel:   cancel,
		dur:      time.Duration(duration),
	}
}

func readRomFile(romFilename string) []byte {
	var rom []byte
	if romFilename == "" {
		panic(fmt.Sprintf("No ROM file specified"))
	}
	rom, err := ioutil.ReadFile(romFilename)
	if err != nil {
		panic(fmt.Sprintf("Failed to read the ROM file at \"%s\" (%v)", romFilename, err))
	}
	return rom
}

func (gb *Gameboy) runFrame(gui gui, end time.Time) {
	// The Game Boy clock runs at 4.194304MHz
	// Each loop iteration below represents one machine cycle
	// One machine cycle is 4 clock cycles
	// Each LCD frame is 17556 machine cycles
	for mtick := 0; mtick < 17556; mtick++ {
		gb.dispatch.ExecuteMachineCycle()
		gb.memory.ExecuteMachineCycle()
		gb.lcd.EndMachineCycle()
		timerInterruptRequested := gb.timer.EndMachineCycle()
		if timerInterruptRequested {
			gb.memory.IF |= 0x04
		}
	}
	gb.lcd.FrameEnd()
	if gui != nil {
		gui.DrawFrame(gb.lcd.Frame)
	}
	time.Sleep(time.Until(end))
	gb.frame++
}

// Run the Gameboy
func (gb *Gameboy) Run(ctx context.Context, gui gui) {
	end := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			end = end.Add(gb.dur)
			gb.runFrame(gui, end)
		}
	}
}

// Time the Gameboy as it runs
func (gb *Gameboy) Time(ctx context.Context, gui gui) {
	end := time.Now()
	for {
		// There are just under 60 frames per second (59.7275) so let's time in blocks of 60 frames
		// On a real Gameboy this would take 1 second
		t0 := time.Now()
		for i := 0; i < 60; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				end = end.Add(gb.dur)
				gb.runFrame(gui, end)
			}
		}
		t1 := time.Now()
		fmt.Println("=========> ", t1.Sub(t0))
	}
}

// ButtonAction turns UI key presses into emulator button presses
func (gb *Gameboy) ButtonAction(b ui.Button, pressed bool) {
	// Start the CPU in case it was stopped waiting for input
	gb.dispatch.Start()
	// Bit 3 - P13 Input Down  or Start    (0=Pressed) (Read Only)
	// Bit 2 - P12 Input Up    or Select   (0=Pressed) (Read Only)
	// Bit 1 - P11 Input Left  or Button B (0=Pressed) (Read Only)
	// Bit 0 - P10 Input Right or Button A (0=Pressed) (Read Only)
	if b == ui.Start {
		if pressed {
			gb.memory.ButtonInput &^= 0x8
		} else {
			gb.memory.ButtonInput |= 0x8
		}
	} else if b == ui.Select {
		if pressed {
			gb.memory.ButtonInput &^= 0x4
		} else {
			gb.memory.ButtonInput |= 0x4
		}
	}
	if b == ui.B {
		if pressed {
			gb.memory.ButtonInput &^= 0x2
		} else {
			gb.memory.ButtonInput |= 0x2
		}
	}
	if b == ui.A {
		if pressed {
			gb.memory.ButtonInput &^= 0x1
		} else {
			gb.memory.ButtonInput |= 0x1
		}
	}
	if b == ui.Down {
		if pressed {
			gb.memory.DirectionInput &^= 0x8
		} else {
			gb.memory.DirectionInput |= 0x8
		}
	} else if b == ui.Up {
		if pressed {
			gb.memory.DirectionInput &^= 0x4
		} else {
			gb.memory.DirectionInput |= 0x4
		}
	}
	if b == ui.Left {
		if pressed {
			gb.memory.DirectionInput &^= 0x2
		} else {
			gb.memory.DirectionInput |= 0x2
		}
	} else if b == ui.Right {
		if pressed {
			gb.memory.DirectionInput &^= 0x1
		} else {
			gb.memory.DirectionInput |= 0x1
		}
	}
}

// Screenshot writes a screenshot to file
func (gb *Gameboy) Screenshot(filename string) {
	f, err := os.Create(filename)
	if err != nil {
		fmt.Println(err)
	}
	defer f.Close()
	err = png.Encode(f, gb.lcd.Frame)
	if err != nil {
		fmt.Println(err)
	}
}

// Faster makes the emulator run faster
func (gb *Gameboy) Faster() {
	gb.dur /= 2
}

// Slower makes the emulator run slower
func (gb *Gameboy) Slower() {
	gb.dur *= 2
}

// Debug enabled for the UI
func (gb *Gameboy) Debug() bool {
	return gb.opts.DebugLCD
}

// Shutdown the emulator
func (gb *Gameboy) Shutdown() {
	gb.cancel()
}
