package main

import "golang.design/x/clipboard"

func initializeClipboard() (bool, error) {
	err := clipboard.Init()
	if err != nil {
		return false, err
	}

	return true, nil
}

func copyText(text string) {
	clipboard.Write(clipboard.FmtText, []byte(text))
}
