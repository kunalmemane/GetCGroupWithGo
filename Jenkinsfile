// Jenkinsfile
pipeline {
    agent {
        kubernetes {
            yaml """
kind: Pod
metadata:
  labels:
    app: jenkins-go-agent
spec:
  containers:
  - name: go-builder
    image: registry.redhat.io/rhel8/go-toolset-1.20:latest
    command: ['cat']
    tty: true
"""
        }
    }
    environment {
        OC_PROJECT = 'my-go-app-dev'
        OC_BUILD_CONFIG = 'my-go-app-build'
        // Changed to use the 'deployment' selector
        OC_DEPLOYMENT = 'my-go-app'
    }
    stages {
        stage('Checkout Source Code') {
            steps {
                script {
                    checkout scm
                    echo "Source code checked out from ${env.GIT_URL} at branch ${env.GIT_BRANCH}"
                }
            }
        }

        stage('Run Go Tests (Optional but Recommended)') {
            steps {
                container('go-builder') {
                    sh 'go mod tidy'
                }
            }
        }

        stage('Trigger OpenShift S2I Build') {
            steps {
                script {
                    openshift.withProject(env.OC_PROJECT) {
                        echo "Starting OpenShift S2I build for ${env.OC_BUILD_CONFIG} in project ${env.OC_PROJECT}..."
                        def build = openshift.build(env.OC_BUILD_CONFIG).start('--from-dir=.')
                        build.logs()
                        build.untilCompletion()
                        echo "OpenShift build completed for ${env.OC_BUILD_CONFIG}."
                    }
                }
            }
        }

        stage('Deploy Application (Automated by ImageStream update)') {
            steps {
                script {
                    openshift.withProject(env.OC_PROJECT) {
                        echo "Waiting for rollout of ${env.OC_DEPLOYMENT} to complete..."
                        // Use 'deployment' as the kind here
                        def deployment = openshift.selector('deployment', env.OC_DEPLOYMENT)
                        if (deployment.exists()) {
                            // Trigger rollout if needed (though ImageStream will likely handle it)
                            // The .latest() method here specifically applies to DeploymentConfig.
                            // For Deployment, you'd typically wait for the deployment to finish
                            // after the image has been pushed and the deployment's image updated.
                            // A simple way to do this is using rollout status.
                            sh "oc rollout status deployment/${env.OC_DEPLOYMENT} --namespace=${env.OC_PROJECT} --timeout=10m"
                            echo "Deployment rollout for ${env.OC_DEPLOYMENT} completed."
                        } else {
                            error "Deployment ${env.OC_DEPLOYMENT} not found in project ${env.OC_PROJECT}."
                        }
                    }
                }
            }
        }

        stage('Verify Deployment (Optional)') {
            steps {
                script {
                    openshift.withProject(env.OC_PROJECT) {
                        def route = openshift.selector('route', env.OC_DEPLOYMENT).object()
                        def app_url = "http://${route.spec.host}"
                        echo "Application URL: ${app_url}"
                        sh "curl -s -f ${app_url}"
                        echo "Application is reachable!"
                    }
                }
            }
        }
    }
    post {
        always {
            cleanWs()
        }
        failure {
            echo "Pipeline failed!"
        }
        success {
            echo "Pipeline succeeded! Go application deployed."
        }
    }
}