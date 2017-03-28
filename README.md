# Function as a service - kubernetes
### Runs a docker image in a kubernetes cluster and prints its output

```sh
$ go get github.com/jwendell/fnkube

# Runs the perl image and prints π with 100 places
$ fnkube -image perl -- perl "-Mbignum=bpi" -wle "print bpi(100)"
3.141592653589793238462643383279502884197169399375105820974944592307816406286208998628034825342117068
```
Run `fnkube -help` for more options and examples.

### Concepts behind it
What `fnkube` does it really simple: It just spawns a [Kubernetes Job](https://kubernetes.io/docs/concepts/jobs/run-to-completion-finite-workloads/) that runs the specified image in a container and grabs its output. After that it deletes the `Job` and `pods` created for this task, unless it's told to not do this (`-cleanup=false` option).
All you need is a kubernetes (or openshift) instance available and permissions to access it.

### As a library
You can incorporate `fnkube` into a bigger application just by importing `"github.com/jwendell/fnkube/pkg/fnkube"` and calling the function `fnkube.Run(options)` whereas `options` is the struct `fnkube.Options`. Take a look at how [main.go](https://github.com/jwendell/fnkube/blob/master/main.go) uses it.
