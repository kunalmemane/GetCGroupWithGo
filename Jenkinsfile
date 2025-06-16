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
                  # IMPORTANT: Verify this image tag exists in your 'openshift' namespace's 'golang' ImageStream
                  # For example: oc get is golang -n openshift -o yaml
                  image: image-registry.openshift-image-registry.svc:5000/openshift/golang:1.20-ubi9
                  command:
                    - cat
                  tty: true
              # Removed explicit volumeMounts/volumes as Kubernetes plugin usually handles workspace automatically
              '''
    }
  }

  // options {
  //     ansiColor('xterm')
  //     timestamps()
  //     buildDiscarder(logRotator(numToKeepStr: '10'))
  // }

  parameters {
      // THIS IS THE PRIMARY FIX: APP_NAME should be 'my-go-app', matching your BuildConfig/Deployment name
      string(name: 'APP_NAME', defaultValue: 'my-go-app', description: 'Name of the OpenShift application (e.g., my-go-app)')
      string(name: 'NAMESPACE', defaultValue: 'my-go-app-dev', description: 'OpenShift project/namespace for the application')
      string(name: 'IMAGE_TAG', defaultValue: "build-${env.BUILD_NUMBER}", description: 'Image tag to push and deploy')
  }

  environment {
      REGISTRY = "image-registry.openshift-image-registry.svc:5000"
      // Construct the full image path using parameters for consistency
      IMAGE_FULL_PATH = "${REGISTRY}/${params.NAMESPACE}/${params.APP_NAME}:${params.IMAGE_TAG}"
      GOCACHE = "/tmp/go-build" // This is typically handled by the S2I builder or within the agent container
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
          // sh 'go test ./...' // Uncomment if you have actual unit tests
          echo "No testing required for this app"
        }
      }
    }

    stage('Build binary') {
      steps {
        container('go-builder') {
          // These chown/mkdir commands are often not needed when using S2I builder images
          // as they set up the environment appropriately for building within the correct user context.
          // sh 'mkdir -p /opt/app-root/src /tmp/src && \
          //     chown -R 1001:0 /opt/app-root /tmp/src'
          // sh 'mkdir /.cache'
          // sh 'chmod -R 777 /.cache'

          // Assuming your Go app source is at the root of the checked out directory
          sh "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/${params.APP_NAME} ."
          sh 'ls -la bin/'
          echo 'Build Successful...'
        }
      }
    }

    stage('Build & Push Image') {
      steps {
        container('go-builder') {
          script {
            // Debugging: Print out the parameter values being used
            echo "DEBUG: Building with APP_NAME: ${params.APP_NAME}, NAMESPACE: ${params.NAMESPACE}, IMAGE_TAG: ${params.IMAGE_TAG}"

            openshift.withCluster() {
              openshift.withProject("${params.NAMESPACE}") {
                // IMPORTANT: The 'oc new-build' command should ideally be run only once, outside the Jenkinsfile
                // (e.g., via oc apply -f go-app-build-config.yaml).
                // If you already have your 'my-go-app-build' BuildConfig, you just need to trigger it.
                // The 'if' condition below would create a BuildConfig named 'my-go-app' if it doesn't exist,
                // which might be different from your existing 'my-go-app-build'.
                // If you intend for Jenkins to CREATE the BuildConfig, ensure 'params.APP_NAME'
                // exactly matches the name you want the BuildConfig to have (e.g., 'my-go-app').
                // If not, remove the 'if' block and just use the openshift.selector to trigger.

                // For simplicity and to match previous steps where you applied 'go-app-build-config.yaml',
                // we will assume the BuildConfig already exists and just trigger it.
                // If you truly want to create it on the fly, uncomment the 'if' block and ensure consistency.

                // if (!openshift.selector("bc", params.APP_NAME).exists()) {
                //   // This command creates a BuildConfig named 'params.APP_NAME' (e.g., 'my-go-app')
                //   sh "oc new-build --name=${params.APP_NAME} --image=registry.access.redhat.com/ubi8/go-toolset --binary=true --strategy=source --to=${params.APP_NAME}:${params.IMAGE_TAG}"
                // }

                // Trigger the existing BuildConfig. Use params.APP_NAME as the BuildConfig name.
                openshift.selector("bc", params.APP_NAME).startBuild("--from-dir=.", "--follow", "--wait")
                echo "OpenShift BuildConfig '${params.APP_NAME}' build completed."
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
                // The BuildConfig already pushes to APP_NAME:latest if the output in BC is set to 'APP_NAME:latest'.
                // This tag command ensures APP_NAME:latest is updated if the BuildConfig output tag is different.
                // If your BuildConfig outputs directly to 'my-go-app:latest', this might be redundant.
                sh "oc tag ${params.NAMESPACE}/${params.APP_NAME}:${params.IMAGE_TAG} ${params.NAMESPACE}/${params.APP_NAME}:latest --overwrite"
                echo "Image tagged as latest."
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
                // Use the correct image path for the Deployment
                // Ensure your Deployment is type 'Deployment' (apps/v1) and not 'DeploymentConfig' (apps.openshift.io/v1)
                // If it's DeploymentConfig, change 'dc' to 'deploymentconfig' in the selector.
                if (!openshift.selector("deployment", params.APP_NAME).exists()) {
                  // This creates a new Deployment and Service if they don't exist
                  // It uses the IMAGE_FULL_PATH (e.g., image-registry.../my-go-app-dev/my-go-app:build-1)
                  // For a clean cluster, this is okay for first-time setup.
                  sh "oc new-app --name=${params.APP_NAME} --image=${IMAGE_FULL_PATH} --port=8080"
                  sh "oc expose svc/${params.APP_NAME}"
                  echo "New application '${params.APP_NAME}' deployed and exposed."
                } else {
                  // This triggers a new rollout by updating the image of an existing Deployment
                  sh "oc set image deployment/${params.APP_NAME} ${params.APP_NAME}=${IMAGE_FULL_PATH} --overwrite"
                  echo "Deployment image updated for '${params.APP_NAME}'."
                }

                // Wait for the Deployment to rollout. Selector for Deployment is 'deployment'.
                openshift.selector("deployment", params.APP_NAME).rollout().status("-w")
                echo "Deployment rollout completed for '${params.APP_NAME}'."

                // Ensure the route exists before attempting to get its host
                openshift.selector("route", params.APP_NAME).untilExists()
                def route = openshift.selector('route', params.APP_NAME).object()
                def app_url = "http://${route.spec.host}"
                echo "Application deployed to: ${app_url}"
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