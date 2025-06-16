pipeline { 
  agent { 
    kubernetes { 
      yaml ''' 
            apiVersion: v1 
            kind: Pod 
            metadata:
              labels:
                app: jenkins-go-agent
            spec: 
              containers: 
                - name: go-builder 
                  image: image-registry.openshift-image-registry.svc:5000/openshift/golang:1.18-ubi9
                  command: 
                    - cat 
                  tty: true 
                  volumeMounts: 
                    - name: workspace-volume 
                      mountPath: /workspace 
                  volumes: 
                    - name: workspace-volume 
                      emptyDir: {} 
              ''' 
    } 
  }

  // options { 
  //     ansiColor('xterm') 
  //     timestamps() 
  //     buildDiscarder(logRotator(numToKeepStr: '10')) 
  // }
  parameters { 
      string(name: 'APP_NAME', defaultValue: 'my-go-app-dev', description: 'Name of the OpenShift application') 
      string(name: 'NAMESPACE', defaultValue: 'my-go-app-dev', description: 'OpenShift project/namespace') 
      string(name: 'IMAGE_TAG', defaultValue: "build-${env.BUILD_NUMBER}", description: 'Image tag to push and deploy') 
  }

  environment { 
      REGISTRY = "image-registry.openshift-image-registry.svc:5000" 
      IMAGE      = "${REGISTRY}/${params.NAMESPACE}/${params.APP_NAME}:${params.IMAGE_TAG}" 
  }
  
  stages { 
    stage('Checkout') { 
      steps { 
        checkout scm 
        echo "Source code checked out from ${env.GIT_URL} at branch ${env.GIT_BRANCH}"

      } 
    }

    stage('Unit tests') {
      steps {
        container('go-builder') {
          // sh 'go test ./...'
          echo "No testing required for this app"
        }
      }
    }

    stage('Build binary') {
      steps {
        container('go-builder') {
          sh 'mkdir -p /opt/app-root/src /tmp/src && \
              chown -R 1001:0 /opt/app-root /tmp/src'
          sh 'mkdir /.cache'
          sh 'chmod -R 777 /.cache'
          sh 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/${APP_NAME} .'
        }
      }
    }

    stage('Build & Push Image') {
      steps {
        container('go-builder') {
          script {
            openshift.withCluster() {
              openshift.withProject("${params.NAMESPACE}") {
                if (!openshift.selector("bc", params.APP_NAME).exists()) {
                  sh "oc new-build --name=${params.APP_NAME} --image=registry.access.redhat.com/ubi8/go-toolset --binary=true --strategy=source --to=${params.APP_NAME}:${params.IMAGE_TAG}"
                }
                openshift.selector("bc", params.APP_NAME).startBuild("--from-dir=.", "--follow", "--wait")
              }
            }
          }
        }
      }
    }

    stage('Tag image as latest') {
      steps {
        container('go-builder') {
          script {
            openshift.withCluster() {
              openshift.withProject("${params.NAMESPACE}") {
                sh "oc tag ${params.NAMESPACE}/${params.APP_NAME}:${params.IMAGE_TAG} ${params.NAMESPACE}/${params.APP_NAME}:latest --overwrite"
              }
            }
          }
        }
      }
    }

    stage('Deploy to OpenShift') {
      steps {
        container('go-builder') {
          script {
            openshift.withCluster() {
              openshift.withProject("${params.NAMESPACE}") {
                if (!openshift.selector("dc", params.APP_NAME).exists()) {
                  sh "oc new-app --name=${params.APP_NAME} --image=${IMAGE} --port=8080"
                  sh "oc expose svc/${params.APP_NAME}"
                } else {
                  sh "oc set image dc/${params.APP_NAME} ${params.APP_NAME}=${IMAGE} --overwrite"
                }
                openshift.selector("dc", params.APP_NAME).rollout().status("-w")
              }
            }
          }
        }
      }
    }
  }

  post { 
      success { 
          echo ":white_check_mark: Deployment successful!" 
        } 
      failure { 
          echo ":x: Build or deployment failed." 
        } 
  } 
}
