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
              '''
    }
  }

  environment {
    APP_NAME = "my-go-app"
    PROJECT_NAME = "my-go-app-dev"
    IMAGE_TAG = "build-${env.BUILD_NUMBER}"
    BUILDCONFIG_NAME = 'cgroup-with-go'

    REGISTRY = "image-registry.openshift-image-registry.svc:5000"
    // Construct the full image path using parameters for consistency
    IMAGE_FULL_PATH = "${REGISTRY}/${params.NAMESPACE}/${params.APP_NAME}:${params.IMAGE_TAG}"
    GOCACHE = "/tmp/go-build" // This is typically handled by the S2I builder or within the agent container
  }

  stages {

    //Child Stage
    stage('Checkout') {
      steps {
        checkout scm
        echo "Source code checked out from ${env.GIT_URL} at branch ${env.GIT_BRANCH}"
      }
    }

    //Child Stage
    // stage('Build binary') {
    //   steps {
    //     container('go-builder') {
    //       sh "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/${params.APP_NAME} ."
    //       sh 'ls -la bin/'
    //       echo 'Build Successful...'
    //     }
    //   }
    // }

    //Child Stage
    stage('Check BuildConfig') {
      steps {
        script {
          openshift.withCluster() {
            openshift.withProject(PROJECT_NAME) {

              echo "Looking for '${BUILDCONFIG_NAME}' if it exists in project '${PROJECT_NAME}'."

              def bc = openshift.selector("bc", BUILDCONFIG_NAME)
              echo "${bc}"



              if (bc.exists()) {
                echo "✅ BuildConfig '${BUILDCONFIG_NAME}' exists in project '${PROJECT_NAME}'."
              } else {
                echo "❌ BuildConfig '${BUILDCONFIG_NAME}' does NOT exist in project '${PROJECT_NAME}'."
                error("BuildConfig '${BUILDCONFIG_NAME}' not found.")
              }
            }
          }
        }
      }
    }

  // Finish parent stage
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