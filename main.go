package main

import (
	"fmt"
	"os"

	"flag"

	"github.com/jwendell/fnkube/pkg/fnkube"
)

func main() {
	namespace := flag.String("namespace", "", "Which namespace we should use. If not provided, a new one will be created. Be aware that in most kubernetes/openshift instances the creation of namespaces is restricted to cluster administrators.")
	image := flag.String("image", "", "Docker image to run on the kubernetes instance")
	timeout := flag.Int("timeout", 120, "Timeout (in seconds) to wait for the job to complete (0 to wait indefinitely)")
	cleanup := flag.Bool("cleanup", true, "Delete all created artifacts (including the namespace, if we created it) after finishing the job")
	insecure := flag.Bool("insecure", false, "Allow insecure TLS communication to kubernetes API server")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Runs a docker image in a kubernetes cluster and prints its output

Usage: fnkube [OPTIONS] -image <IMAGE> -- COMMANDS

Examples:

  # Runs the perl image and prints π with 100 places
  fnkube -image perl -- perl "-Mbignum=bpi" -wle "print bpi(100)"

  # Same as above, but specifies a namespace and do not delete stuff (useful for debugging)
  fnkube -namespace myproject -cleanup=false -image perl -- perl "-Mbignum=bpi" -wle "print bpi(100)"

Options:
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	if len(*image) == 0 {
		fmt.Fprintf(os.Stderr, "Missing image. Specify one with -image\n\n")
		flag.Usage()
		os.Exit(1)
	}

	command := flag.Args()
	if len(command) == 0 {
		fmt.Fprintf(os.Stderr, "Missing the command to run\n\n")
		flag.Usage()
		os.Exit(1)
	}

	options := &fnkube.Options{
		Namespace: *namespace,
		Image:     *image,
		Command:   command,
		Timeout:   *timeout,
		Cleanup:   *cleanup,
		Auth:      fnkube.AuthInfo{Insecure: *insecure},
	}
	stdout, stderr, err := fnkube.Run(options)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Fprint(os.Stdout, stdout)
	fmt.Fprint(os.Stderr, stderr)
}
