package clipboard

import (
	"time"

	"golang.design/x/clipboard"
)

func Initialize() (bool, error) {
	err := clipboard.Init()
	if err != nil {
		return false, err
	}
	return true, nil
}

func Copy(text string) {
	clipboard.Write(clipboard.FmtText, []byte(text))
}

func Read() string {
	return string(clipboard.Read(clipboard.FmtText))
}

func Monitor(interval time.Duration, callback func(string)) {
	go func() {
		lastText := Read()
		for {
			currentText := Read()
			if currentText != "" && currentText != lastText {
				lastText = currentText
				callback(currentText)
			}
			time.Sleep(interval)
		}
	}()
}
