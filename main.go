package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
)

var logger *log.Logger
var GitCommit, GitState, BuildDate string

const (
	// X, Y, Z axes (2 bytes per axis)
	BufferSize    = 6
	SampleMaxSize = 0xFFFF
	StreamBufferSize = 4098
	LSM6DSL_SampleDiscard = 3
)

type Axis struct {
	AxisName   string
	Total      int16
	NumSamples int
	//TotalSamples int16
	MinVal    int16
	MaxVal    int16
	pCount    int
	nCount    int
	NumTamper int
	//Tamper       bool
	Alert   Tone
	UpaVals Upa
}

type Upa struct {
	MaxSamples int
	MaxWindow  int
	Threshold  int
}

type ASwitch struct {
	CurAxis string
}

func newAxis(name string, alert Tone, upa Upa) *Axis {
	a := Axis{
		AxisName:   name,
		Total:      0,
		NumSamples: 0,
		MinVal:     0,
		MaxVal:     0,
		pCount:     0,
		nCount:     0,
		NumTamper:  0,
		//Tamper:     false,
		Alert:   alert,
		UpaVals: upa,
	}

	return &a
}

func newUpa(maxSamples int, maxWindow int, thres int) *Upa {
	u := Upa{
		MaxSamples: maxSamples,
		MaxWindow:  maxWindow,
		Threshold:  thres,
	}
	return &u
}

func (self *Axis) upaInit() {
	// Only generate our Min/Max value when one does not exist and once we have enough samples
	if self.NumSamples >= self.UpaVals.MaxSamples && self.MinVal == 0 && self.MaxVal == 0 {
		var average int16 = self.Total / int16(self.NumSamples)
		self.MinVal = average - int16(self.UpaVals.Threshold)
		self.MaxVal = average + int16(self.UpaVals.Threshold)
		//self.Tamper = false
		logger.Printf("%s: Avg: %d (Thres: %d), Min: %d, Max: %d\n", self.AxisName, average, self.UpaVals.Threshold, self.MinVal, self.MaxVal)
	}
}

func (self *Axis) axisAppend(val int16) {
	self.Total += val
	self.NumSamples += 1
	//self.TotalSamples += 1
}

func (self *Axis) checkTrigger(val int16) {
	// Min/Max shouldn't exist yet anyway but double check to make sure
	if self.NumSamples < self.UpaVals.MaxSamples {
		return
	}
	if val > self.MaxVal {
		self.pCount++
		self.nCount = 0
		logger.Printf("%s: Max, p: %d, val: %d > max: %d, count: %d\n", self.AxisName, self.pCount, val, self.MaxVal, self.NumSamples)
	} else if val < self.MinVal {
		self.pCount = 0
		self.nCount++
		logger.Printf("%s: Min, n: %d, val: %d < min %d, count: %d\n", self.AxisName, self.nCount, val, self.MinVal, self.NumSamples)
	} else {
		self.pCount = 0
		self.nCount = 0
	}

	// Once tamper triggered, recalculate rolling average
	if self.pCount >= self.UpaVals.MaxWindow || self.nCount >= self.UpaVals.MaxWindow {
		self.NumTamper++
		fmt.Printf("%s axis **** TAMPER #: %d **** \n", self.AxisName, self.NumTamper)
		playTone(self.Alert)
		//self.Tamper = true
		self.MinVal = 0
		self.MaxVal = 0
		self.pCount = 0
		self.nCount = 0
		self.NumSamples = 0
		self.Total = 0
	}
}

func (self *Axis) performUpa(val int16) {
	self.axisAppend(val)
	self.upaInit()
	self.checkTrigger(val)
}

func (self *Axis) outputStats() {
	//logger.Printf("%s: Samples: %d, Tamper: %d\n", self.AxisName, self.TotalSamples, self.NumTamper)
	logger.Printf("%s: Tamper: %d\n", self.AxisName, self.NumTamper)
}

func (self *ASwitch) nextAxis() {
	switch self.CurAxis {
	case "X":
		self.CurAxis = "Y"
	case "Y":
		self.CurAxis = "Z"
	case "Z":
		self.CurAxis = "X"
	}
}

func convertToLeS16(msb uint16, lsb uint16) int16 {
	n := msb<<8 + lsb
	return int16(n)
}

func calcIntThreshold(divisor int) int {
	// Only accept 1-10% +- of value, round down
	if divisor <= 10 && divisor >= 1 {
		val := int(math.Floor(float64(divisor) / 100 * float64(SampleMaxSize)))
		return val
	}
	return 1
}

func loggingMode(vFlag *bool, t Tone) {
	logger = log.New(ioutil.Discard, "Tamper: ", log.LstdFlags)
	if *vFlag {
		logger.SetOutput(os.Stdout)
		logger.Printf("Verbose logging enabled\n")
		logger.Printf("Version: %s-%s, Date: %s\n", GitCommit, GitState, BuildDate)
		outputDefaults(t)
	}
}

func main() {

	dirPath := flag.String("path", "/dev/iio:device1", "Path to the iio device")
	thresholdVal := flag.Int("threshold", 1, "Sensitivity of tamper detection (1-10)")
	numSamples := flag.Int("avg-samples", 20, "Number of samples used in average calculation")
	numWindow := flag.Int("max-window", 10, "Number of consecuative samples before tamper")
	verboseFlag := flag.Bool("verbose", false, "Enable verbose logging")
	toneFlag := flag.Bool("play-tone", false, "Play tone on tamper trigger")
	tonePath := flag.String("tone-path", "/usr/share/sounds/j2emu/9_ATSMTone.wav", "Path to tamper alert tone")

	flag.Parse()

	t := newTone(*tonePath, *toneFlag)
	u := newUpa(*numSamples, *numWindow, calcIntThreshold(*thresholdVal))

	loggingMode(verboseFlag, *t)

	// iio buffer check
	fifoStatus, err := queryGyroFifoEnabled()
	if err != nil {
		logger.Println("Unable to determine FIFO operational, quitting")
		os.Exit(1)
	}

	// If iio buffer not already running, manually set values and bring up
	if !fifoStatus {
		err := tamperStartup()
		if err != nil {
			logger.Println("Unable to preamble iio FIFO")
		}
	}

	fmt.Printf("Running j4tamper-upa... \n")
	fmt.Printf("\tDevice: %s, Sample Average: %d\n", *dirPath, *numSamples)
	fmt.Printf("\tMax Window: %d, Sensitivity: %d\n", *numWindow, *thresholdVal)

	x := newAxis("X", *t, *u)
	y := newAxis("Y", *t, *u)
	z := newAxis("Z", *t, *u)

	// Start samples at X (assumed)
	p := ASwitch{
		CurAxis: "X",
	}

	file, err := os.Open(*dirPath)
	if err != nil {
		logger.Println("Open error: ", err)
		return
	}
	defer file.Close()

	buffer := make([]byte, StreamBufferSize)
	var offset uint = 0

	for {
		bytesread, err := file.Read(buffer[offset:])
		offset = 0

		// err value can be io.EOF, which means that we reached the end of
		// file, don't think it will ever happen but in case
		if err != nil {
			if err != io.EOF {
				logger.Println("EOF: ", err)
			}
			logger.Println("Read error: ", err)
			break
		}

		newbuff := []byte(buffer[:bytesread])

		// expect our buffer size (6)
		// each axis represents 2 bytes in the following order:
		//  Gx, Gy, Gz
		// Data is little endian s16 therefore it is the following:
		// e.g. [1110100 1010110 11010100 11111100 11110101 1]
		//  Gx = 01010110 01110100 => 0x5674 => 22132
		//  Gy = 11111100 11010100 => 0xFCD4 => -812
		//  Gz = 00000001 11110101 => 0x01F5 => 501
		if bytesread != BufferSize {
			logger.Printf("Skipping... Bytes read: %d, Expected %d\n", bytesread, BufferSize)
			continue
		}

		for ; len(newbuff) >= 2; {
			val := convertToLeS16(uint16(newbuff[1]), uint16(newbuff[0]))
			// This whole thing can be improved.  Using slice index for axis mapper is better.
			switch p.CurAxis {
			case "X":
				x.performUpa(val)
				p.nextAxis()
				newbuff = newbuff[2:]
			case "Y":
				y.performUpa(val)
				p.nextAxis()
				newbuff = newbuff[2:]
			case "Z":
				z.performUpa(val)
				p.nextAxis()
				newbuff = newbuff[2:]
			}
		}
		// Return values in FIFO back to slice array if it is not of 2 bytes long.
		copy(buffer, newbuff)
		offset = uint(len(newbuff))
	}

	//	x.outputStats()
	//	y.outputStats()
	//	z.outputStats()
}
