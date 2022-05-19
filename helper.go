package main

import (
	"os"
	"os/exec"
)

const (
	udevPath        = "/var/volatile/udev/"
	xAxisEn         = "gyro_x_enable"
	yAxisEn         = "gyro_y_enable"
	zAxisEn         = "gyro_z_enable"
	fifoWatermark   = "gyro_fifo_watermark"
	fifoLength      = "gyro_fifo_length"
	fifoEn          = "gyro_fifo_enable"
	bufferWatermark = "12"
	bufferLength    = "72"
)

type Tone struct {
	TonePath string
	Enabled  bool
}

func newTone(path string, enb bool) *Tone {
	t := Tone{
		TonePath: path,
		Enabled:  enb,
	}
	return &t
}

func outputDefaults(alertTone Tone) {
	logger.Printf("Watermark: %s, Length: %s\n", bufferWatermark, bufferLength)
	logger.Printf("Play tone: %t\n", alertTone.Enabled)
	logger.Printf("Tone path: %s\n", alertTone.TonePath)
}

func playTone(alertTone Tone) {
	if alertTone.Enabled {
		cmd := exec.Command("aplay", alertTone.TonePath)
		cmd.Run()
	}
}

func enableGyroAxes() error {
	err := os.WriteFile(udevPath+xAxisEn, []byte("1"), 0644)
	if err != nil {
		logger.Println("Unable to enable: ", err)
		return err
	}
	logger.Printf("Enabled Gx\n")

	err = os.WriteFile(udevPath+yAxisEn, []byte("1"), 0644)
	if err != nil {
		logger.Println("Unable to enable: ", err)
		return err
	}
	logger.Printf("Enabled Gy\n")

	err = os.WriteFile(udevPath+zAxisEn, []byte("1"), 0644)
	if err != nil {
		logger.Println("Unable to enable: ", err)
		return err
	}
	logger.Printf("Enabled Gz\n")
	return nil
}

func setBufferDefaults() error {
	// Length must be set before watermark because watermark must be < length
	err := os.WriteFile(udevPath+fifoLength, []byte(bufferLength), 0644)
	if err != nil {
		logger.Println("Unable to set FIFO: ", err)
		return err
	}
	logger.Printf("Set Length: %s\n", bufferLength)
	err = os.WriteFile(udevPath+fifoWatermark, []byte(bufferWatermark), 0644)
	if err != nil {
		logger.Println("Unable to set FIFO: ", err)
		return err
	}
	logger.Printf("Set Watermark: %s\n", bufferWatermark)
	return nil
}

func enableGyroFifo() error {
	err := os.WriteFile(udevPath+fifoEn, []byte("1"), 0644)
	if err != nil {
		logger.Println("Unable to enable FIFO: ", err)
		return err
	}
	logger.Printf("Enabling FIFO...\n")
	return nil
}

func disableGyroFifo() error {
	err := os.WriteFile(udevPath+fifoEn, []byte("0"), 0644)
	if err != nil {
		logger.Println("Unable to disable FIFO: ", err)
		return err
	}
	logger.Printf("Disabling FIFO...\n")
	return nil
}

func tamperStartup() error {
	err := disableGyroFifo()
	if err != nil {
		return err
	}
	err = enableGyroAxes()
	if err != nil {
		return err
	}
	err = setBufferDefaults()
	if err != nil {
		return err
	}
	err = enableGyroFifo()
	if err != nil {
		return err
	}
	return nil
}

func queryGyroFifoEnabled() (bool, error) {
	data, err := os.ReadFile(udevPath + fifoEn)
	if err != nil {
		logger.Println("Unable to query FIFO: ", err)
		return false, err
	}
	if string(data[:1]) == "1" {
		logger.Println("FIFO: Enabled")
		return true, nil
	}
	logger.Println("FIFO: Disabled")
	return false, nil
}
