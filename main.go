package main

import (
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	rpio "github.com/stianeikeland/go-rpio"
	"sync"
	"time"
)

var (
	pinEnd       = rpio.Pin(20)
	pinBeginning = rpio.Pin(21)
	pinPWM       = rpio.Pin(13)
	pinDir1      = rpio.Pin(24)
	pinDir2      = rpio.Pin(25)
	pinRelay     = rpio.Pin(16)
	sense        = "stopped"
	mux          sync.Mutex
)

func main() {
	var webFlag = flag.Bool("w", false, "if the web interface should be started")
	var operation = flag.String("o","cycle", "operation to execute: cycle, forward, backward, stop")
	flag.Parse()

	err := rpio.Open()
	if err != nil {
		panic(err)
	}
	defer func() {
		rpio.StopPwm()
		rpio.Close()
	}()

	pinEnd.Input()
	pinBeginning.Input()
	pinBeginning.PullUp()
	pinEnd.PullUp()
	pinPWM.Pwm()
	pinDir1.Output()
	pinDir2.Output()
	pinRelay.Output()
	pinDir1.Low()
	pinDir2.Low()
	pinRelay.Low()

	if *webFlag {
		r := gin.Default()

		r.Static("/static", "./static")
		r.GET("/forward", func(c *gin.Context) {
			go motorForwardWithCheck()
			c.JSON(200, gin.H{
				"message": "ok",
			})
		})
		r.GET("/backward", func(c *gin.Context) {
			go motorBackwardWithCheck()
			c.JSON(200, gin.H{
				"message": "ok",
			})
		})
		r.GET("/stop", func(c *gin.Context) {
			go motorStop()
			c.JSON(200, gin.H{
				"message": "ok",
			})
		})
		r.GET("/cycle", func(c *gin.Context) {
			go startCycle()
			c.JSON(200, gin.H{
				"message": "ok",
			})
		})
		r.Run()
	} else {
		switch(*operation){
		case "forward":
			motorForwardWithCheck()
		case "backward":
			motorBackwardWithCheck()
		case "stop":
			motorStop()
		default:
			startCycle()
		}
	}

}


func startCycle() {
	motorForward()
	for pinEnd.Read() != rpio.Low {
		time.Sleep(1 * time.Millisecond)
	}
	motorStop()
	motorBackward()
	for pinBeginning.Read() != rpio.Low {
		time.Sleep(1 * time.Millisecond)
	}
	motorStop()
	motorForward()
	motorStop()
	fillTub()
}

func fillTub(){
	pinRelay.High()
	time.Sleep(60 * time.Second)
	pinRelay.Low()
}

func motorForwardWithCheck(){
	motorForward()
	for pinEnd.Read() != rpio.Low && sense!="stopped" && sense!="stopping"{
		time.Sleep(1 * time.Millisecond)
	}
	motorStop()
}
func motorForward() {
	mux.Lock()
	//we are already running forward
	if sense == "forward" {
		mux.Unlock()
		return
	}
	//we do not want to force the motor so we stop first
	if sense != "stopped" {
		mux.Unlock()
		motorStop()
		return
	}
	pinDir1.High()
	pinDir2.Low()
	sense = "forward"
	mux.Unlock()
	motorIncreasing()
}

func motorBackwardWithCheck(){
	motorBackward()
	for pinBeginning.Read() != rpio.Low && sense!="stopped" && sense!="stopping"{
		time.Sleep(1 * time.Millisecond)
	}
	motorStop()
}
func motorBackward() {
	mux.Lock()
	//we are already running backward
	if sense == "backward" {
		mux.Unlock()
		return
	}
	//we do not want to force the motor so we stop first
	if sense != "stopped" {
		mux.Unlock()
		motorStop()
		return
	}
	pinDir1.Low()
	pinDir2.High()
	sense = "backward"
	mux.Unlock()
	motorIncreasing()
}

func shouldStop() bool {
	return (sense == "forward" && pinEnd.Read() == rpio.Low) ||
		(sense == "backward" && pinBeginning.Read() == rpio.Low) ||
		(sense == "stopped" || sense == "stopping")
}

func motorIncreasing() {
	if shouldStop() {
		return
	}
	pinPWM.Freq(100000)
	pinPWM.DutyCycle(0, 100)
	for i := 10; i < 100; i++ {
		pinPWM.DutyCycle(uint32(i), 100)
		if shouldStop() {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("Max value reached")
}

func motorStop() {
	mux.Lock()
	defer mux.Unlock()
	if sense == "stopped" || sense == "stopping" {
		return
	}
	sense = "stopping"
	for i := 50; i > 0; i -= 5 {
		pinPWM.DutyCycle(uint32(i), 100)
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("Min value reached")
	pinDir1.Low()
	pinDir2.Low()
	time.Sleep(1 * time.Second)
	sense = "stopped"
}
