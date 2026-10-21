package server

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"strings"
	"unicode/utf16"
)

const startupTaskDescription = "Jumpcode server for the signed-in user"

type startupTaskDefinition struct {
	XMLName          xml.Name `xml:"Task"`
	XMLNS            string   `xml:"xmlns,attr"`
	Version          string   `xml:"version,attr"`
	RegistrationInfo struct {
		Description string `xml:"Description"`
	} `xml:"RegistrationInfo"`
	Triggers struct {
		LogonTrigger struct {
			Enabled bool   `xml:"Enabled"`
			UserID  string `xml:"UserId"`
		} `xml:"LogonTrigger"`
	} `xml:"Triggers"`
	Principals struct {
		Principal struct {
			ID        string `xml:"id,attr"`
			UserID    string `xml:"UserId"`
			LogonType string `xml:"LogonType"`
			RunLevel  string `xml:"RunLevel"`
		} `xml:"Principal"`
	} `xml:"Principals"`
	Settings struct {
		MultipleInstancesPolicy    string `xml:"MultipleInstancesPolicy"`
		DisallowStartIfOnBatteries bool   `xml:"DisallowStartIfOnBatteries"`
		StopIfGoingOnBatteries     bool   `xml:"StopIfGoingOnBatteries"`
		AllowHardTerminate         bool   `xml:"AllowHardTerminate"`
		AllowStartOnDemand         bool   `xml:"AllowStartOnDemand"`
		Enabled                    bool   `xml:"Enabled"`
		RunOnlyIfIdle              bool   `xml:"RunOnlyIfIdle"`
		ExecutionTimeLimit         string `xml:"ExecutionTimeLimit"`
		RestartOnFailure           struct {
			Interval string `xml:"Interval"`
			Count    int    `xml:"Count"`
		} `xml:"RestartOnFailure"`
	} `xml:"Settings"`
	Actions struct {
		Context string `xml:"Context,attr"`
		Exec    struct {
			Command   string `xml:"Command"`
			Arguments string `xml:"Arguments"`
		} `xml:"Exec"`
	} `xml:"Actions"`
}

func startupTaskXML(userSID, program, powershell string, specs []string, lang string) ([]byte, error) {
	var task startupTaskDefinition
	task.XMLNS = "http://schemas.microsoft.com/windows/2004/02/mit/task"
	task.Version = "1.2"
	task.RegistrationInfo.Description = startupTaskDescription
	task.Triggers.LogonTrigger.Enabled = true
	task.Triggers.LogonTrigger.UserID = userSID
	task.Principals.Principal.ID = "Author"
	task.Principals.Principal.UserID = userSID
	task.Principals.Principal.LogonType = "InteractiveToken"
	task.Principals.Principal.RunLevel = "LeastPrivilege"
	task.Settings.MultipleInstancesPolicy = "IgnoreNew"
	task.Settings.AllowHardTerminate = true
	task.Settings.AllowStartOnDemand = true
	task.Settings.Enabled = true
	task.Settings.ExecutionTimeLimit = "PT0S"
	task.Settings.RestartOnFailure.Interval = "PT1M"
	task.Settings.RestartOnFailure.Count = 3
	task.Actions.Context = "Author"
	task.Actions.Exec.Command = powershell
	var listenArgs strings.Builder
	for _, spec := range specs {
		fmt.Fprintf(&listenArgs, " --listen %s", psLiteral(spec))
	}
	script := fmt.Sprintf("& %s --lang %s server%s --task-run; exit $LASTEXITCODE", psLiteral(program), psLiteral(lang), listenArgs.String())
	task.Actions.Exec.Arguments = "-NoLogo -NoProfile -NonInteractive -WindowStyle Hidden -EncodedCommand " + encodedPowerShell(script)
	data, err := xml.MarshalIndent(task, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), data...), nil
}

func psLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func encodedPowerShell(script string) string {
	units := utf16.Encode([]rune(script))
	data := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(data[i*2:], unit)
	}
	return base64.StdEncoding.EncodeToString(data)
}
