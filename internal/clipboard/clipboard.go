package clipboard

import (
	"context"

	"golang.design/x/clipboard"
)

func Initialize() (bool, error) {
	err := clipboard.Init()
	if err != nil {
		return false, err
	}
	return true, nil
}

func Copy(text string) error {
	_, err := clipboard.Write(context.Background(), clipboard.FmtText, []byte(text))
	return err
}

func Read() (string, error) {
	text, err := clipboard.Read(context.Background(), clipboard.FmtText)
	return string(text), err
}
