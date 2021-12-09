# Kpack reconciler

This is a proof-of-concept reconciler for
[kpack](https://github.com/pivotal/kpack)

# How to run it

1. Build it

```
make build-linux
```

2. Target a cluster, for example create an empty kind cluster: `

```
kind create cluster --name kyma
```

3. Install kpack 0.4.1 using the mothership. For the POC purposes the
   mothership would use a [fork of the kyma
   repository](https://github.com/danail-branekov/kyma) that contains the kpack
   helm chart

```
KUBECONFIG=~/.kube/config ./bin/mothership-linux local --version kpack-0.4.1 --components kpack
```

After the mothership is done, kpack will be installed in the `kyma-kpack`
namespace.

4. Bump kpack to 0.4.3 using the mothership. For demonstration purposes the
   reconcile action limits the version bump to be within a single major
   version. This however might not be needed in real-life scenarios.

```
KUBECONFIG=~/.kube/config ./bin/mothership-linux local --version kpack-0.4.3 --components kpack
```

After the mothership is done old kpack pods will be terminated and new ones
with the 0.4.3 kpack images would be created.

5. Delete kpack using the mothership

```
KUBECONFIG=~/.kube/config ./bin/mothership-linux local --components kpack -d
```

After the mothership is done, the `kyma-kpack` namespace is going to be
terminated, thus kpack uninstalled.

# Food for thought

-   When performing an update of the rendered manifest using the kyma kubernetes
    client you would normally get conflicts, because the deployed objects have
    accumulated some state and are different from the ones in the manifest. In
    the POC we handle this by introducing a service interceptor that takes this
    in consideration. This might not be sufficient in the real world and more
    interceptors for other kinds of objects might be beeded. This might soon
    become unwieldy. We think it would be better if the kyma reconciler
    framework handles this for all reconcilers thus reducing their complexity.
    This might be done by using a deployment management solution like
    [Helm](https://helm.sh/) or
    [kapp](https://github.com/vmware-tanzu/carvel-kapp). Helm is currently only
    being used as a templating engine leaving the state management to the
    reconcilers. From our experience `kapp` handles corner cases better than
    `helm` making it a better fit for automation usage.

-   `kpack` does not distribute a helm template, it just ships a rendered
    template. For the purpose of the spike, we created a helm template
    ourselves in order to be consistent with the rest of the reconcilers. There
    are other implementation possibilities though, e.g.

    -   the reconcile action could always download the rendered template itself
        and just apply it
    -   the released rendered template could be a plain yaml resource in
        `kyma-reconciler/.workspace/main/resources/kpack` that is just applied
        by the action

-   the [helm
    chart](https://github.com/danail-branekov/kyma/tree/kpack-0.4.1/resources/kpack)
    provided for the poc is extremely simple and naive. Its complexity would we
    driven by further kyma requirements.
