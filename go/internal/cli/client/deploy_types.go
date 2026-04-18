package clientcmd

type manifestOptions struct {
	remoteHost     string
	installDir     string
	installDirSet  bool
	trojanPort     string
	trojanUser     string
	trojanPassword string
	tunMode        string
	tunModeSet     bool
	force          bool
}

type runtimeOptions struct {
	remoteHost string
	deployPort string
	serverHost string
	ciphertext []byte
}

type deployOptions struct {
	manifest manifestOptions
	runtime  runtimeOptions
}
