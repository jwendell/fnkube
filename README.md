# Runs a docker image in a kubernetes cluster and prints its output

```sh
$ fnkube -image perl -- perl "-Mbignum=bpi" -wle "print bpi(100)"
3.141592653589793238462643383279502884197169399375105820974944592307816406286208998628034825342117068
$ 
```
