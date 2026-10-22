package server

import (
	"bytes"
	"encoding/xml"
	"strings"
)

// 通过 COM 传递 Unicode XML，不依赖系统代码页或临时 XML 文件。
func taskDefinitionXML(sid, executable, workingDir, language string, specs []string) string {
	escape := func(s string) string {
		var b bytes.Buffer
		xml.EscapeText(&b, []byte(s))
		return b.String()
	}
	args := []string{"--lang", language, "server"}
	for _, spec := range specs {
		args = append(args, "--listen", spec)
	}
	args = append(args, "--task-run")
	// 参数均由受限的语言、IPv4/auto 和端口值组成，不含路径或 shell 输入。
	return `<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo><Description>Jumpcode desktop server</Description></RegistrationInfo>
  <Triggers><LogonTrigger><Enabled>true</Enabled><UserId>` + escape(sid) + `</UserId></LogonTrigger></Triggers>
  <Principals><Principal id="User"><UserId>` + escape(sid) + `</UserId><LogonType>InteractiveToken</LogonType><RunLevel>LeastPrivilege</RunLevel></Principal></Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings><StopOnIdleEnd>false</StopOnIdleEnd><RestartOnIdle>false</RestartOnIdle></IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand><Enabled>true</Enabled>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <RestartOnFailure><Interval>PT1M</Interval><Count>3</Count></RestartOnFailure>
  </Settings>
  <Actions Context="User"><Exec><Command>` + escape(executable) + `</Command><Arguments>` + escape(strings.Join(args, " ")) + `</Arguments><WorkingDirectory>` + escape(workingDir) + `</WorkingDirectory></Exec></Actions>
</Task>`
}
