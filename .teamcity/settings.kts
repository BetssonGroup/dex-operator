import jetbrains.buildServer.configs.kotlin.v2019_2.*

/*
The settings script is an entry point for defining a TeamCity
project hierarchy. The script should contain a single call to the
project() function with a Project instance or an init function as
an argument.

VcsRoots, BuildTypes, Templates, and subprojects can be
registered inside the project using the vcsRoot(), buildType(),
template(), and subProject() methods respectively.

To debug settings scripts in command-line, run the

    mvnDebug org.jetbrains.teamcity:teamcity-configs-maven-plugin:generate

command and attach your debugger to the port 8000.

To debug in IntelliJ Idea, open the 'Maven Projects' tool window (View
-> Tool Windows -> Maven Projects), find the generate task node
(Plugins -> teamcity-configs -> teamcity-configs:generate), the
'Debug' option is available in the context menu for the task.
*/

version = "2019.2"

project {

    params {
        param("VCSRepositoryURL", "ssh://git@edgecharlie.corpsson.com:7999/iac/dex-operator.git")
    }
    subProject(DexOperator)
}

object DexOperator : Project({
    name = "Dex Operator Docker Build"
    buildType(BuildDocker)
})

object BuildDocker : BuildType({
    name = "Build Docker Images"
    allowExternalStatus = true
    vcs { root(AbsoluteId("GenericGitSsh"))}
    steps {
        script {
            name = "Docker Build and Pushild"
            workingDir = "."
            scriptContent = "make IMG=localhost:5000/dex-operator:v0.1.0 docker-build docker-push"
        }
    }
    requirements {
        startsWith("cloud.amazon.agent-name-prefix", "Ubuntu-20.04")
        moreThan("teamcity.agent.hardware.memorySizeMb", "16000")
    }
})
