# Installing Manual Approval Gate

NOTE:- You need to install [tektoncd/pipeline](https://github.com/tektoncd/pipeline/blob/main/docs/install.md)

1. Manual Approval Gate Installation
   *  On Kubernetes
       ```
       make apply
       ```
   * On Openshift
      ```
      make TARGET=openshift apply
      ```

2. Install a pipelineRun which has approval task as one of the task in the pipelin
   - For example

     ```yaml
      apiVersion: tekton.dev/v1
      kind: PipelineRun
      metadata:
        generateName: test-
      spec:
        pipelineSpec:
          tasks:
            - name: before
              taskSpec:
                steps:
                  - image: busybox
                    name: before
                    script: echo before wait
            - name: wait
              taskRef:
                apiVersion: openshift-pipelines.org/v1alpha1
                kind: ApprovalTask
              params:
                - name: approvers
                  value:
                    - foo
                    - bar
                    - kubernetes-admin
                - name: numberOfApprovalsRequired
                  value: 2
                - name: description
                  value: Approval Task Rocks!!!
              timeout: 2m
              runAfter: ['before']
            - name: after
              taskSpec:
                steps:
                  - image: busybox
                    name: after
                    script: echo after wait
              runAfter: ['wait']

     ```
   Install the above pipelineRun

    ```shell
    kubectl create -f <pipeline.yaml>
    ```

    **NOTE** :- _Once the pipelineRun is started after the execution of first task is done, it will create a customRun for that approval task and the pipeline will be in `pending` state till it gets the approval from the user. The name of the approvalTask is the same of customRun which is created.
As of today only `"approve"` and `"reject"` are supported. If user passes the approval as `"approve"` then pipeline will proceed to execute the further tasks and if `"reject"` is provided then in that case it will fail the pipeline_


3. Now `approve/reject` the approval task by using `kubectl edit` command by updating the `input` field under `approvers` section fo your username

  To get the approvalTask name you can use this command

  ```bash
  kubectl get approvaltask
  ```

   **NOTE** :- If you are using a kind cluster and you need to approve/reject for that particular user then in that case you can run the following command

   ```bash
    kubectl edit approvaltask <approvalTaskname> --as=<username>
   ```