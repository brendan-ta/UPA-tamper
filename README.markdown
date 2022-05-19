# Tamper detection for LSM6DSL IMU equipped devices

This is a simple working example which showcases the possibility of using method known as Unipolar
Persistent Acceleration (UPA) detection to determine whether the device has been tampered with.
i.e. the device has been under a significant angular change in either its X, Y, or Z axes.

## Requirements

- golang
- Device equipped with LSM6DSL sensor
- Linux OS with Industrial I/O support with LSM6DSL drivers

## Notes

- Designed as a proof of concept see the UPA algorithm at work.
- Prints to stdout
- Plays tone using `aplay` when tamper has triggered
- Using rudimentary process to keep track of axis sequence from popped FIFO sample
- sysfs symlink bindings were used for shorter directory paths.
- Tested with the following LSM6DSL configuration
    - Angular rate change: 125dps
    - Sensor ODR: 52hz
    - FIFO ODR: 52hz
    - FIFO contiunous mode
- Linux FIFO IIO configuration:
    - Length: 72
    - Watermark: 12

## Sample Frames

Length and watermark were chosen for specific reasons.  We are assuming that all 3 axes (X, Y, and Z) will
be present in detection which means each sample will be of 6 bytes (2 bytes per axis).  Therefore the watermark
at 12 should represent two full samples pushed from the FIFO before an IRQ is alerted.  Length of 72 means that
6 sample frames can fit in nicely and all values are evenly divisible with each other, even if we decide to do
2 axes (4 samples per frame) or 1 axis (2 samples per frame).

FIFO data pattern:

    Gx, Gy, Gz, Gx, Gy, Gz...

In this example, accelerometer is disabled hence there are no accelerometer values in the FIFO.

![LSM6DSL-axes-orientation](https://github.com/brendan-ta/UPA-tamper/raw/master/images/lsm6dslaxes.png)

## Sensitivity

Sensor ODR directly relates to the sensitivity of the tamper algorithm if nothing else is touched.  This is due to
the rate at which the samples are obtained.  Assuming the ODR is fixed and not adjusted the following determines its
sensitivity.

Average samples: the number of samples it uses to obtain the average before UPA calculation
Max window: the number of consecuative samples that exceed the minimum or maximum before a tamper is issued
Threshold: this is a percentage of the max available value.  Because it is an signed 16bit number the max value is 65535.

## Building

    go build

## Usage

    Usage of ./UPA-tamper:
      -avg-samples int
            Number of samples used in average calculation (default 20)
      -max-window int
            Number of consecuative samples before tamper (default 10)
      -path string
            Path to the iio device (default "/dev/iio:device1")
      -play-tone
            Play tone on tamper trigger
      -threshold int
            Sensitivity of tamper detection (1-10) (default 1)
      -tone-path string
            Path to tamper alert tone (default "/usr/share/sounds/j2emu/9_ATSMTone.wav")
      -verbose
            Enable verbose logging


## Resources

- [DSL Datasheet](https://www.st.com/resource/en/datasheet/lsm6dsl.pdf)
- [DSL Application Note AN5040](https://www.st.com/resource/en/application_note/an5040-lsm6dsl-alwayson-3d-accelerometer-and-3d-gyroscope-stmicroelectronics.pdf)
- [Tamper Sensing Using Low-Cost Accelerometers KTH Master Thesis, Spring 2012 - Tobias Rådeskog (XR-EE-SB 2012:003)](http://www.diva-portal.org/smash/get/diva2:538868/FULLTEXT01.pdf)

### Unipolar Peristent Acceleration (UPA) detector

A strongly reoccurring property of benign signals, is the tendency to not keep far from one side of
equilibrium for very long. Something very simple is therefore proposed - the UPA detector (Unipolar
Persistent Acceleration), which in this application seems highly characteristic for malignant movement
and displacement.

For a single channel (each channel is handled separately), it is mathematically defined like this:
Let the sample n from a gravity compensated single channel be denoted An
Let the threshold function

![upa-threshold-function](https://github.com/brendan-ta/UPA-tamper/raw/master/images/thresholdfunction.png)

where τu is a configurable threshold parameter (in g's - or in practical implementation -
accelerometer resolution counts).
The UPA value is then the value k in the longest observed sequence in a series starting with sample n
which fulfills the conditions:

![upa-value](https://github.com/brendan-ta/UPA-tamper/raw/master/images/upavalue.png)

In most programming languages this is very easy to implement on the fly, essentially by just increasing a
counter with one whenever 

![upa-detection](https://github.com/brendan-ta/UPA-tamper/raw/master/images/upadetection.png)

If Tr (Acurrent) = 0 the counter resets to zero immediately and if the counter goes over a certain threshold, which can be called
τp (measured in number of samples) an alert is raised.

What this means qualitatively is that the sensitivity of the detector is set by two thresholds, both the
acceleration threshold τu and the persistence threshold τp - (how many consecutive samples may be
on one side of the threshold before an alert is raised). Each channel is handled separately, occurrence on
only one channel is enough to trigger the detector.


## Comments

Code is average, just designed purely to test the UPA algorithm.  A final solution made for an embedded system
was made following this which is of a completely different approach (using libiio/iiod rather than reading from sysfs),
UPA calculations are similar in design.
