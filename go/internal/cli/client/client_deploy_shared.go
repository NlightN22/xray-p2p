package clientcmd

import (
	"os"
	"os/exec"
	"time"
)

const (
	defaultRemoteInstallDir = `C:\xp2p`
	defaultLocalInstallDir  = `C:\xp2p-client`
	defaultPingTarget       = "10.0.10.10"
)

type deployOptions struct {
	remoteHost       string
	sshUser          string
	sshPort          string
	serverHost       string
	serverPort       string
	trojanUser       string
	trojanPassword   string
	remoteInstallDir string
	remoteConfigDir  string
	localInstallDir  string
	localConfigDir   string
	saveLinkPath     string
	packageOnly      bool
	packagePath      string
}

type sshTarget struct {
	user string
	host string
	port string
}

var (
	lookPathFunc               = exec.LookPath
	executablePathFunc         = os.Executable
	sshCommandFunc             = runSSHCommand
	scpCommandFunc             = runSCPCommand
	startProcessFunc           = startDetachedProcess
	stopLocalProcessFunc       = stopProcess
	stopRemoteFunc             = stopRemoteService
	writeFileFunc              = os.WriteFile
	sleepFunc                  = time.Sleep
	runPingCommandFunc         = runPingCommand
	ensureRemoteBinaryFunc     = ensureRemoteBinary
	prepareRemoteServerFunc    = prepareRemoteServer
	installLocalClientFunc     = installLocalClient
	startRemoteServerFunc      = startRemoteServer
	startLocalClientFunc       = startLocalClient
	runPingCheckFunc           = runPingCheck
	releaseProcessHandleFunc   = releaseDetachedProcess
	promptStringFunc           = promptString
	buildDeploymentPackageFunc = buildDeploymentPackage
)
