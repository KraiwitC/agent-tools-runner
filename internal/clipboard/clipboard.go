package clipboard

import "golang.design/x/clipboard"

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
