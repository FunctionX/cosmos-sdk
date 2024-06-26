package simulation

import (
	"fmt"
	"os"
	"path"
	"time"
)

// log writer
type LogWriter interface {
	AddEntry(OperationEntry)
	PrintLogs()
}

// LogWriter - return a dummy or standard log writer given the testingmode
func NewLogWriter(testingmode bool) LogWriter {
	if !testingmode {
		return &DummyLogWriter{}
	}

	return &StandardLogWriter{}
}

// log writer
type StandardLogWriter struct {
	OpEntries []OperationEntry `json:"op_entries" yaml:"op_entries"`
}

// add an entry to the log writer
func (lw *StandardLogWriter) AddEntry(opEntry OperationEntry) {
	lw.OpEntries = append(lw.OpEntries, opEntry)
}

// PrintLogs - print the logs to a simulation file
func (lw *StandardLogWriter) PrintLogs() {
	f := createLogFile()
	defer f.Close()

	for i := 0; i < len(lw.OpEntries); i++ {
		writeEntry := fmt.Sprintf("%s\n", (lw.OpEntries[i]).MustMarshal())
		_, err := f.WriteString(writeEntry)
		if err != nil {
			panic("Failed to write logs to file")
		}
	}
}

func createLogFile() *os.File {
	var f *os.File

	fileName := fmt.Sprintf("%d.log", time.Now().UnixMilli())
	folderPath := path.Join(os.ExpandEnv("$HOME"), ".simapp", "simulations")
	filePath := path.Join(folderPath, fileName)

	err := os.MkdirAll(folderPath, os.ModePerm)
	if err != nil {
		panic(err)
	}

	f, err = os.Create(filePath)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Logs to writing to %s\n", filePath)

	return f
}

// dummy log writer
type DummyLogWriter struct{}

// do nothing
func (lw *DummyLogWriter) AddEntry(_ OperationEntry) {}

// do nothing
func (lw *DummyLogWriter) PrintLogs() {}
