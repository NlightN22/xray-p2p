package root

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

var logTimestampPattern = regexp.MustCompile(`(?m)^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z`)
var expiryDaysPattern = regexp.MustCompile(`expires in -?\d+ days`)

var humanBaselineDigests = map[string]string{
	"xp2p client forward list":        "24b6a5a715b8d1b780d26624b1b7c2bbbee188620a13ea8572ba53302074ec96",
	"xp2p client dns-forward list":    "b9c5213e8c7184e024fd0fabae91122e4667927ded6e39d7860033b5f76d5349",
	"xp2p client dns-forward add":     "8d7e936b0a7ee106cac26f1ec162c97e3e203cc252d9e5626e9b300188ab099a",
	"xp2p client dns-forward remove":  "972e9c90f9d46f0bc412c4cfabac84e90863449b51d781a8cd071639a32e008e",
	"xp2p client group list":          "3c6baf022ec14a91f7dcbdafc5cd65d894632c32fee867ea5d6aac62ffaa86b4",
	"xp2p client list":                "f25a2b635bf72a624160b09d317dfa2039f864bec129f904c3be47ba657ba9a7",
	"xp2p client mode":                "968ae1af4737e7b5d7f3d97fa9a0e9754b896d318d5d045f69bbc32d3d19de4d",
	"xp2p client remove":              "511a1376bc32709276903b6e4de5a9753a707bce8abe712cc5ab09419197e03f",
	"xp2p client obs":                 "443ab8889e2d23dcc1a37ada68e1dccdbdbca7f864faaedc97d972902d2eb4f7",
	"xp2p client redirect list":       "893c3fcb29a471ccbbdc0a4da587f1317f1006b8aa352fb4393ea3c85f535d54",
	"xp2p client reverse list":        "b57a7e69bdad9f92150eed005bf011ee62f0d67f6fbddb0cac21326606a31994",
	"xp2p client service status":      "371e959b24229eb7b84ce97c7a94d7f8a7e550f99be5fdb1eda557323718c17b",
	"xp2p client service restart":     "ef7cb4a2cd23af4e8fc92549e103556df3e12aca07a3dbb0a662a8b936f24c9f",
	"xp2p client service start":       "fa550b6eb2fc8a82f2bf2920310df054602c758e99e6fd15ed234dc7684d8521",
	"xp2p client service stop":        "e78d6dbd67c103c1465d105eccedff8acf6cd6db5ea357b360524556df367f88",
	"xp2p client state":               "51c9957525f118b5d222236fdcba0657eaa4dad563136f35338d4d8f6819a327",
	"xp2p client subscription offers": "79ce972c4121aafdafbf1642f861e934f5324a7eb32fd85bb02ad9bef13cadb2",
	"xp2p client subscription status": "eef7a656111a23a6b0d41b97bfb0a703b867ff34e66be1b61c6490206ba18f85",
	"xp2p heartbeat contract":         "f40c05e3d46a73d7e1445db6fedc953a65f8ee65af751ee730c4efe1dda81791",
	"xp2p nat-redirect list":          "192df5a2fa7407e711cdbf08955880814baf7a0955da786adaf4ab72006b8db9",
	"xp2p nat-redirect add":           "4379792348e26b4a8d934563c6b42a386a272ee5b766e7a912c628451f1b553c",
	"xp2p nat-redirect remove":        "b126d344d9a5d8247e32a1292f506c8ec69c2f175f379f1de606c17a339b1b08",
	"xp2p server cert state":          "532d4c194a31463f133167602cfa44e229300c6256942012a975f40fd534178e",
	"xp2p server cert set":            "90c4677a298f492705cfa09931372c802706fe388c58866d0c6dcfc149f98526",
	"xp2p server dns-forward list":    "b9c5213e8c7184e024fd0fabae91122e4667927ded6e39d7860033b5f76d5349",
	"xp2p server dns-forward add":     "8d7e936b0a7ee106cac26f1ec162c97e3e203cc252d9e5626e9b300188ab099a",
	"xp2p server dns-forward remove":  "972e9c90f9d46f0bc412c4cfabac84e90863449b51d781a8cd071639a32e008e",
	"xp2p server forward list":        "24b6a5a715b8d1b780d26624b1b7c2bbbee188620a13ea8572ba53302074ec96",
	"xp2p server ha channel inspect":  "6bb913387a8d4edb924d8f48d315e8082068edd54194a61acf5902a1fa909857",
	"xp2p server ha channel list":     "cee0f237909a87ead4a4f5f4528c2ae19ccf0bdc5f25cec182f743157c42c264",
	"xp2p server ha group inspect":    "89ce36dac794085ab8fa6fce71d5cdccb26681217fd5ff4c99a46800f0d838ce",
	"xp2p server ha member list":      "982a11442a1a101afd94d7e0b10377ec07af6269a914dd1bbb69e0ee3bab548f",
	"xp2p server ha peer list":        "a3abe78d397913d4277a3252eebdf13531b6d905cf260eb90054f1da5b6e8442",
	"xp2p server ha redirect list":    "00331c7b022c272b1164e16dbe2dee3f445a7688defbda9b9b2cd7af7c737a3f",
	"xp2p server ha status":           "a0a4bd864f2990898eb636b5cf1998a667ef389f9a059fbc0968ed755e1a31e7",
	"xp2p server identity status":     "8f3ed9ec2fe05b0c8624b6b05dbd95e2df22c74214a50deec755be2765e94078",
	"xp2p server mode":                "f33560fe6128eba15bfb8233fe395d407c37bd8ba9925d809db80fbe9308f179",
	"xp2p server profile":             "805ddf2a7da1ac11323c884c2ef4b0d77435824ce86baec85413022b51fc60d0",
	"xp2p server redirect list":       "893c3fcb29a471ccbbdc0a4da587f1317f1006b8aa352fb4393ea3c85f535d54",
	"xp2p server reverse list":        "4799165844fa2105b0244c352f8a55e5b200a4dbbea2799b4643f2a3f42bbd1a",
	"xp2p server service status":      "616d1d1949eeff53787c3ceaf2fd7900c4bdcee13ae136cd9ff7cf79a722d2ab",
	"xp2p server service restart":     "5f488bf42ff98dd65602200a6a00a74b98606762013606a0726071670eb56045",
	"xp2p server service start":       "c069d8e5dbe7c006cce69b901b65d0af2694a9775c192118943dd643efdf45db",
	"xp2p server service stop":        "bbf83053268e8db339d6e13a946f4b0f357c17557c6df8b7eda6517bf9604ce6",
	"xp2p server remove":              "fe8194fc3fb7cdd9e79bc2e9291dc8ca4028024b9f2ba4e24134a94b41b9f632",
	"xp2p server state":               "d8b16883f86a9a421514310ddd628dd74a185030a20a0fde0280e1a6ff94361d",
	"xp2p server user list":           "280ba819543defc726b5159efdac99740dc3dd80dec02931778e1bb9ecb130bc",
}

var humanPlatformBaselineDigests = map[string]map[string]string{
	"xp2p client remove": {
		"linux": "cd26ffa56db354024860603f29c39547727fc59a9045f2e49d45b15f9bc71b31",
	},
	"xp2p server cert set": {
		"linux": "9015a140ee7d4749ff6d3e37b2f435e43e8862820bf9e302f06a18d1e0d747f0",
	},
	"xp2p server remove": {
		"linux": "78c93050715e9f36a371952723815fc2400d0389b75e54de6b2011f03b84081c",
	},
}

type contractExecution struct {
	stdout   string
	stderr   string
	err      error
	exitCode int
}

func assertHumanBaseline(t *testing.T, path, stdout, stderr string) {
	t.Helper()
	normalized := normalizeHumanOutput(stdout) + "\x00" + normalizeHumanOutput(stderr)
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(normalized)))
	expected, ok := humanBaselineDigests[path]
	if platformExpected := humanPlatformBaselineDigests[path][runtime.GOOS]; platformExpected != "" {
		expected = platformExpected
		ok = true
	}
	if !ok {
		t.Fatalf("missing exact human baseline for %s: digest=%s normalized=%q", path, digest, normalized)
	}
	if digest != expected {
		t.Fatalf("human baseline changed for %s: got=%s want=%s normalized=%q", path, digest, expected, normalized)
	}
}

func normalizeHumanOutput(value string) string {
	for _, root := range []string{os.Getenv("XP2P_LOG_ROOT"), os.Getenv("XP2P_CONFIG_ROOT")} {
		if root != "" {
			value = strings.ReplaceAll(value, root, "<ROOT>")
			value = strings.ReplaceAll(value, filepath.ToSlash(root), "<ROOT>")
		}
	}
	value = logTimestampPattern.ReplaceAllString(value, "<TIME>")
	return expiryDaysPattern.ReplaceAllString(value, "expires in <DAYS> days")
}

func executeContractCase(args []string, cancel bool) contractExecution {
	return executeContractCaseContext(context.Background(), args, cancel)
}

func executeContractCaseContext(ctx context.Context, args []string, cancel bool) contractExecution {
	allArgs := append([]string{"--json"}, args...)
	cmd := NewCommandForArgs(allArgs)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(allArgs)
	if cancel {
		var cancelContext context.CancelFunc
		ctx, cancelContext = context.WithCancel(ctx)
		cancelContext()
	}
	cmd.SetContext(ctx)
	processStdout, processStderr, err := captureProcessStreams(cmd.Execute)
	return contractExecution{
		stdout: stdout.String() + processStdout,
		stderr: stderr.String() + processStderr,
		err:    err, exitCode: ProcessExitCode(err),
	}
}

func assertReadOnlyEdgeCases(t *testing.T, result map[string]any, stdout, stderr string) {
	t.Helper()
	if stderr != "" {
		t.Fatalf("JSON diagnostic stream is not empty: %q", stderr)
	}
	raw := stdout + fmt.Sprintf("%v", result)
	for _, forbidden := range []string{"\x1b[", "matrix-secret", "PRIVATE KEY", "password:", "token="} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("executable edge check found %q in JSON output: %q", forbidden, stdout)
		}
	}
	assertNoCredentialFields(t, result, "result")
}

func assertNoCredentialFields(t *testing.T, value any, path string) {
	t.Helper()
	switch item := value.(type) {
	case map[string]any:
		for key, child := range item {
			normalized := strings.ToLower(key)
			for _, forbidden := range []string{"password", "secret", "token", "private_key", "credential"} {
				if strings.Contains(normalized, forbidden) {
					t.Fatalf("credential-shaped field %s.%s is present", path, key)
				}
			}
			assertNoCredentialFields(t, child, path+"."+key)
		}
	case []any:
		for index, child := range item {
			assertNoCredentialFields(t, child, fmt.Sprintf("%s[%d]", path, index))
		}
	case string:
		if strings.Contains(item, "trojan://") || strings.Contains(item, "vless://") {
			t.Fatalf("credential link is present at %s", path)
		}
	}
}
