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

type manifestOptions struct {
	remoteHost     string
	installDir     string
	trojanPort     string
	trojanUser     string
	trojanPassword string
}

type runtimeOptions struct {
	remoteHost      string
	sshUser         string
	sshPort         string
	serverHost      string
	remoteConfigDir string
	localInstallDir string
	localConfigDir  string
	packageOnly     bool
}

type deployOptions struct {
	manifest    manifestOptions
	runtime     runtimeOptions
	packagePath string
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
