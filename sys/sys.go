package sys

import (
	"encoding/json"
	"os"
	"time"
)

var (
	cachedSys      *Type
	lastSysModTime time.Time
)

type Type struct {
	InitialAcceptCode bool `json:"initialAcceptCode"`
}

func Get() (*Type, error) {
	info, err := os.Stat("./sys/sys.json")
	if err != nil {
		return nil, err
	}

	if cachedSys == nil || info.ModTime().After(lastSysModTime) {
		// 파일 읽기 및 파싱
		data, err := os.ReadFile("./sys/sys.json")
		if err != nil {
			return nil, err
		}

		var sys Type
		err = json.Unmarshal(data, &sys)
		if err != nil {
			return nil, err
		}

		cachedSys = &sys
		lastSysModTime = info.ModTime()
	}

	return cachedSys, nil
}

// Edit 반드시 Get 후에 호출할 것
func Edit(s *Type) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile("./sys/sys.json", data, 0644)
	if err != nil {
		return err
	}

	return nil
}
