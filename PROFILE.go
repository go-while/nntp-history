package history

import (
	"log"
	"os"
	"runtime/pprof"
)

var (
	CPUProfile bool // set before boot
	//MEMProfile bool // set before boot
)

func (his *HISTORY) startCPUProfile() (*os.File, error) {

	cpuProfileFile, err := os.Create("cpu.pprof.out")
	if err != nil {
		log.Printf("ERROR startCPUProfile err1='%v'", err)
		return nil, err
	}
	if err := pprof.StartCPUProfile(cpuProfileFile); err != nil {
		log.Printf("ERROR startCPUProfile err2='%v'", err)
		return nil, err
	}
	log.Printf("startCPUProfile")
	return cpuProfileFile, nil
}

func (his *HISTORY) stopCPUProfile(cpuProfileFile *os.File) {
	if cpuProfileFile == nil {
		log.Printf("ERROR stopCPUProfile cpuProfileFile=nil")
		return
	}
	log.Printf("stopCPUProfile")
	pprof.StopCPUProfile()
	cpuProfileFile.Close()
}
