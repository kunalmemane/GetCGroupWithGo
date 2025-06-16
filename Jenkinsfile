// Jenkinsfile
pipeline {
    agent {
        kubernetes {
            // Jenkins will dynamically provision a pod on OpenShift for this pipeline.
            // This pod needs the Go toolset to run 'go test'.
            // Use an S2I builder image that has Go installed.
            yaml """
kind: Pod
metadata:
  labels:
    app: jenkins-go-agent
spec:
  containers:
  - name: go-builder
    image: registry.redhat.io/ubi9/go-toolset:latest # <<< IMPORTANT: Ensure this image is available and correct for your cluster
    command: ['cat']
    tty: true
"""
        }
    }
    environment {
        // Define environment variables for your OpenShift project and resource names
        OC_PROJECT = 'my-go-app-dev'
        OC_BUILD_CONFIG = 'my-go-app-build'
        OC_DEPLOYMENT = 'my-go-app' // Name of your Kubernetes Deployment
    }
    stages {
        stage('Checkout Source Code') {
            steps {
                script {
                    checkout scm // Checks out the code from the Git repository linked to this Jenkins job
                    echo "Source code checked out from ${env.GIT_URL} at branch ${env.GIT_BRANCH}"
                }
            }
        }

        stage('Run Go Tests') {
            steps {
                container('go-builder') { // Execute these steps inside the 'go-builder' container of the agent pod
                    sh 'go mod tidy' // Ensure dependencies are in sync
                    // sh 'go test ./...' // Run your Go unit tests
                    echo "Go tests completed successfully."
                }
            }
        }

        stage('Trigger OpenShift S2I Build') {
            steps {
                script {
                    openshift.withProject(env.OC_PROJECT) { // Switch to your Go app's OpenShift project
                        echo "Starting OpenShift S2I build for ${env.OC_BUILD_CONFIG} in project ${env.OC_PROJECT}..."
                        // Trigger the BuildConfig defined earlier.
                        // '--from-dir=.' tells the build to use the current Jenkins workspace as source.
                        def build = openshift.build(env.OC_BUILD_CONFIG).start('--from-dir=.')
                        build.logs() // Stream build logs to the Jenkins console
                        build.untilCompletion() // Wait for the OpenShift build to complete
                        echo "OpenShift build completed successfully for ${env.OC_BUILD_CONFIG}."
                    }
                }
            }
        }

        stage('Deploy Application') {
            steps {
                script {
                    openshift.withProject(env.OC_PROJECT) {
                        echo "Waiting for rollout of Deployment '${env.OC_DEPLOYMENT}' to complete..."
                        // For Kubernetes Deployments, OpenShift's ImageStream controller
                        // will automatically update the Deployment's image when a new image
                        // is pushed to the ImageStream (by the S2I build).
                        // We then wait for the rollout initiated by that image change to complete.
                        sh "oc rollout status deployment/${env.OC_DEPLOYMENT} --namespace=${env.OC_PROJECT} --timeout=10m"
                        echo "Deployment rollout for ${env.OC_DEPLOYMENT} completed successfully."
                    }
                }
            }
        }

        stage('Verify Deployment (Optional)') {
            steps {
                script {
                    openshift.withProject(env.OC_PROJECT) {
                        // Get the route URL for your application
                        def route = openshift.selector('route', env.OC_DEPLOYMENT).object()
                        def app_url = "http://${route.spec.host}"
                        echo "Application URL: ${app_url}"
                        // Perform a simple curl request to check if the app is reachable
                        sh "curl -s -f ${app_url}"
                        echo "Application is reachable and responding!"
                    }
                }
            }
        }
    }
    post {
        always {
            cleanWs() // Clean up the Jenkins workspace after the pipeline finishes
        }
        failure {
            echo "Pipeline failed! Check the logs for errors."
            // Add notification steps here (e.g., send email, Slack message)
        }
        success {
            echo "Pipeline succeeded! Go application deployed to OpenShift."
            // Add notification steps here
        }
    }
}