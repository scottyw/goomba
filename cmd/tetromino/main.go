package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime/pprof"

	"github.com/scottyw/tetromino/pkg/gb"
	"github.com/scottyw/tetromino/pkg/ui"
)

func main() {

	// Command line flags
	fast := flag.Bool("fast", true, "When true, Tetromino runs the emulator as fast as possible (true by default)")
	debugCPU := flag.Bool("debugcpu", false, "When true, CPU debugging is enabled")
	debugTimer := flag.Bool("debugtimer", false, "When true, timer debugging is enabled")
	debugLCD := flag.Bool("debuglcd", false, "When true, colour-based LCD debugging is enabled")
	enableTiming := flag.Bool("timing", false, "When true, timing is output every 60 frames")
	enableProfiling := flag.Bool("profiling", false, "When true, CPU profiling data is written to 'cpuprofile.pprof'")
	flag.Parse()

	// CPU profiling
	if *enableProfiling {
		f, err := os.Create("cpuprofile.pprof")
		if err != nil {
			log.Printf("Failed to write cpuprofile.pprof: %v", err)
			return
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			log.Printf("Failed to start cpu profile: %v", err)
			return
		}
		defer pprof.StopCPUProfile()
	}

	rom := flag.Arg(0)
	if rom == "" {
		fmt.Println("No ROM filename was specified")
		os.Exit(1)
	}

	opts := gb.Options{
		RomFilename: rom,
		Fast:        *fast,
		DebugCPU:    *debugCPU,
		DebugTimer:  *debugTimer,
		DebugLCD:    *debugLCD,
	}

	// Run context
	ctx, cancelFunc := context.WithCancel(context.Background())

	// Create the Gameboy emulator
	gameboy := gb.NewGameboy(opts)

	// Create a display
	display, err := ui.NewGLDisplay(gameboy, cancelFunc)
	if err != nil {
		log.Printf("Failed to create display: %v", err)
		return
	}
	defer display.Cleanup()
	gameboy.RegisterDisplay(display)

	// Create speakers
	speakers, err := ui.NewPortaudioSpeakers()
	if err != nil {
		log.Printf("Failed to create speakers: %v", err)
		return
	}
	defer speakers.Cleanup()
	gameboy.RegisterSpeakers(speakers)

	// Start running the emulator
	if *enableTiming {
		gameboy.Time(ctx)
	} else {
		gameboy.Run(ctx)
	}

}
