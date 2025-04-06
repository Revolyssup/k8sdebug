# K8sdebug

## Scope

This tool is created to solve my personal pain points while debugging an application inside K8s environment. This tool is not written from the perspective of a devops engineer but rather from the perspective of a backend application engineer trying to debug issues with the application in k8s environment. This tool is not created to be used directly with production environments. It expects certain privileges that might not be available in production. The audience for this is specifically developers running dev clusters or local clusters while debugging an application related issue. Any future feature that may be added will respect the above fact and not go beyond that scope. There might be mature tools available already that might have some intersection with the features provided here but most of those tools are devops centric. This tool is primarily application centric.

## Features

####  1. Persistent Log analysis

**Problem Addressed:**
Kubernetes does **not** store logs of deleted pods by default. Tools like `kubectl logs` only work for existing pods. Existing solutions (e.g., Loki, Elasticsearch) require complex log aggregation setups.

**Solution:**

k8sdebug runs a daemon to persistently capture logs for all pods in a namespace to a local directory, with features like diffing logs across pods in a deployment.

**Usage**

```bash
k8sdebug logs record start -n <namespace>
```

This will start a dameon process that will record logs from all pods in that namespace and persist it on filesystem (by default in /tmp). Later the logs can be analyzed with following command.

``` bash
k8sdebug logs diff -n <namespace> --type deployment --tail 20(default) <name of deployment>
```

To get the diff between all the pods created under this deployment on the last 20 lines of the logs along with timestamp of each pod.

If there were 5 pods then 4 diffs will be generated one after the other like this.

```bash
pod1 (timestamp)-pod2 (timestamp)
<diff>
pod2(ts) - pod3(ts)
<diff>
.... and so on.
```

```bash
k8sdebug logs show -n <namespace> --type replicaset --tail 20(default) --index 3
(the no of pod chronologically which was created. default to latest)  <name of replicaset>
```

will log the logs of 3rd pod created under this replicaset.

```bash
k8sdebug logs record stop -n <namespace>
```

will stop the daemon process.

```bash
k8sdebug logs setpath <path>
```

can set the default path where files are stored. Defaults to /tmp.

```bash
k8sdebug logs getpath
```

returns the path. The path is stored in a .k8sdebug file in ~ in key value form like

```.env
PATH=/home/ashish/k8sdebug.
```

First time the k8sdebug command is called this file will be created.

## Progress

- [ ] Persistent Log analysis
- [ ] Smart port forwarding
- [ ] In-cluster connectivity testing.
- [ ] `kubebox`
- [ ] Traffic recording and visualisation
