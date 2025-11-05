package clientcmd

import (
	"os"
	"os/exec"
	"time"
)

const (
	defaultRemoteInstallDir = `C:\xp2p`
	defaultLocalInstallDir  = `C:\xp2p-client`
	defaultPingTarget       = "127.0.0.1"
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
	sshBinary       string
	scpBinary       string
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
	startProcessFunc           = startDetachedProcess
	stopLocalProcessFunc       = stopProcess
	sleepFunc                  = time.Sleep
	runPingCommandFunc         = runPingCommand
	installLocalClientFunc     = installLocalClient
	startLocalClientFunc       = startLocalClient
	runPingCheckFunc           = runPingCheck
	releaseProcessHandleFunc   = releaseDetachedProcess
	promptStringFunc           = promptString
	buildDeploymentPackageFunc = buildDeploymentPackage
	ensureSSHPrerequisitesFunc = ensureSSHPrerequisites
	sshCommandFunc             = executeSSHCommand
	sshInteractiveCommandFunc  = executeInteractiveSSHCommand
	scpCopyFunc                = executeSCPCommand
	runRemoteDeploymentFunc    = runRemoteDeployment
)
