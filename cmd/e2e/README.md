### Running the End-To-End Tests

The following environment variables should be set:

1. `E2E_NAMESPACE` is the namespace where the tests should be run.
3. `OPERATOR_ID` is set so that all stacks are only managed by the controller being currently tested.
4. `KUBECONFIG` with the path to the kubeconfig file

To run the tests run the command:

```
go test -parallel $NUM_PARALLEL github.com/zalando-incubator/es-operator/cmd/e2e
```

Over here `$NUM_PARALLEL` can be set to a sufficiently high value which indicates how many
of the parallel type tests can be run concurrently.
