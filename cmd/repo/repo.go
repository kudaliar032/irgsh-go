package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/blankon/irgsh-go/pkg/systemutil"
	"github.com/manifoldco/promptui"
)

func uploadLog(logPath string, id string) {
	// Upload the log to chief
	cmdStr := "curl -v -F 'uploadFile=@" + logPath + "' '" + irgshConfig.Chief.Address + "/api/v1/log-upload?id=" + id + "&type=repo'"
	fmt.Println(cmdStr)
	_, err := systemutil.CmdExec(
		cmdStr,
		"Uploading log file to chief",
		"",
	)
	if err != nil {
		fmt.Println(err.Error())
	}
}

// Main task wrapper
func Repo(payload string) (err error) {
	fmt.Println("##### Submitting the package into the repository")
	in := []byte(payload)
	var raw map[string]interface{}
	json.Unmarshal(in, &raw)

	experimentalSuffix := "-experimental"
	if !raw["isExperimental"].(bool) {
		experimentalSuffix = ""
	}

	logPath := irgshConfig.Repo.Workdir + "/artifacts/"
	logPath += raw["taskUUID"].(string) + "/repo.log"
	go systemutil.StreamLog(logPath)

	cmdStr := fmt.Sprintf(`mkdir -p %s/artifacts && \
	cd %s/artifacts/ && \
	wget %s/artifacts/%s.tar.gz && \
	tar -xvf %s.tar.gz`,
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.Workdir,
		irgshConfig.Chief.Address,
		raw["taskUUID"].(string),
		raw["taskUUID"].(string),
	)
	_, err = systemutil.CmdExec(cmdStr, "Downloading the artifact", logPath)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		uploadLog(logPath, raw["taskUUID"].(string))
		return
	}

	gnupgDir := "GNUPGHOME=" + irgshConfig.Repo.GnupgDir
	if irgshConfig.IsDev {
		gnupgDir = ""
	}
	if raw["isExperimental"].(bool) {
		// Ignore version conflict
		cmdStr = fmt.Sprintf(`cd %s/%s/ && \
		%s reprepro -v -v -v --nothingiserror remove %s \
		$(cat %s/artifacts/%s/*.dsc | grep 'Source:' | cut -d ' ' -f 2)`,
			irgshConfig.Repo.Workdir,
			irgshConfig.Repo.DistCodename+experimentalSuffix,
			gnupgDir,
			irgshConfig.Repo.DistCodename+experimentalSuffix,
			irgshConfig.Repo.Workdir,
			raw["taskUUID"],
		)
		_, err := systemutil.CmdExec(
			cmdStr,
			"This is experimental package, remove any existing package.",
			logPath,
		)
		if err != nil {
			// Ignore err
			fmt.Printf("error: %v\n", err)
		}
	}

	cmdStr = fmt.Sprintf(`cd %s/%s/ && \
	%s reprepro -v -v -v --nothingiserror --component %s includedeb %s %s/artifacts/%s/*.deb`,
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename+experimentalSuffix,
		gnupgDir,
		raw["component"],
		irgshConfig.Repo.DistCodename+experimentalSuffix,
		irgshConfig.Repo.Workdir,
		raw["taskUUID"],
	)

	_, err = systemutil.CmdExec(
		cmdStr,
		"Injecting the deb files from artifact to the repository",
		logPath,
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		uploadLog(logPath, raw["taskUUID"].(string))
		return
	}

	cmdStr = fmt.Sprintf("cd %s/%s/ && reprepro -v -v -v export",
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename,
	)
	_, err = systemutil.CmdExec(
		cmdStr,
		"Initialize the reprepro repository for the first time",
		logPath,
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	uploadLog(logPath, raw["taskUUID"].(string))
	fmt.Println("[ BUILD DONE ]")
	return
}

func InitRepo() (err error) {
	prompt := promptui.Prompt{
		Label:     "Are you sure you want to initialize new repository? Any existing distribution will be flushed.",
		IsConfirm: true,
	}
	result, err := prompt.Run()
	if err != nil {
		return
	}
	if strings.ToLower(result) != "y" {
		return
	}

	// TODO ask for matched distribution name as this command is super dangerous
	// Prepare workdir
	err = os.MkdirAll(irgshConfig.Repo.Workdir, 0755)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("##### Initializing new repository for " + irgshConfig.Repo.DistCodename)

	logPath := irgshConfig.Repo.Workdir + "/init.log"
	go systemutil.StreamLog(logPath)

	repoTemplatePath := "/usr/share/irgsh/reprepro-template"
	if irgshConfig.IsDev {
		cwd, _ := os.Getwd()
		repoTemplatePath = cwd + "/utils/reprepro-template"
	} else if irgshConfig.IsTest {
		dir, _ := os.Getwd()
		repoTemplatePath = dir + "/../utils/reprepro-template"
	}
	cmdStr := fmt.Sprintf("mkdir -p %s && rm -rf %s/%s; cp -R %s %s/%s",
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename,
		repoTemplatePath,
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename,
	)
	_, err = systemutil.CmdExec(cmdStr, "Preparing reprepro template", logPath)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	cmdStr = fmt.Sprintf(`cd %s/%s/conf && cat updates.orig | 
		sed 's/UPSTREAM_NAME/%s/g' | 
		sed 's/UPSTREAM_DIST_CODENAME/%s/g' | 
		sed 's/UPSTREAM_DIST_URL/%s/g' | 
		sed 's/DIST_SUPPORTED_ARCHITECTURES/%s/g' | 
		sed 's/UPSTREAM_DIST_COMPONENTS/%s/g' > updates && rm updates.orig`,
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename,
		irgshConfig.Repo.UpstreamName,
		irgshConfig.Repo.UpstreamDistCodename,
		strings.Replace(irgshConfig.Repo.UpstreamDistUrl, "/", "\\/", -1),
		irgshConfig.Repo.DistSupportedArchitectures,
		irgshConfig.Repo.UpstreamDistComponents,
	)
	_, err = systemutil.CmdExec(
		cmdStr,
		"Populate the reprepro's updates config file with values from irgsh's config.yml",
		logPath,
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	cmdStr = fmt.Sprintf(`cd %s/%s/conf && cat distributions.orig |
		sed 's/DIST_NAME/%s/g' |
		sed 's/DIST_LABEL/%s/g' |
		sed 's/DIST_CODENAME/%s/g' |
		sed 's/DIST_COMPONENTS/%s/g' |
		sed 's/DIST_SUPPORTED_ARCHITECTURES/%s/g' |
		sed 's/DIST_VERSION_DESC/%s/g' |
		sed 's/DIST_VERSION/%s/g' |
		sed 's/DIST_SIGNING_KEY/%s/g' |
		sed 's/UPSTREAM_NAME/%s/g'> distributions && rm distributions.orig`,
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename,
		irgshConfig.Repo.DistName,
		irgshConfig.Repo.DistLabel,
		irgshConfig.Repo.DistCodename,
		irgshConfig.Repo.DistComponents,
		irgshConfig.Repo.DistSupportedArchitectures,
		irgshConfig.Repo.DistVersionDesc,
		irgshConfig.Repo.DistVersion,
		irgshConfig.Repo.DistSigningKey,
		irgshConfig.Repo.UpstreamName,
	)
	_, err = systemutil.CmdExec(
		cmdStr,
		"Populate the reprepro's distributions config file with values from irgsh's config.yml",
		logPath,
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	repositoryPath := strings.Replace(
		irgshConfig.Repo.Workdir+"/"+irgshConfig.Repo.DistCodename,
		"/",
		"\\/",
		-1,
	)
	cmdStr = fmt.Sprintf(`cd %s/%s/conf && \
	cat options.orig | sed 's/IRGSH_REPO_WORKDIR/%s/g' > options && \
	rm options.orig`,
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename,
		repositoryPath,
	)
	_, err = systemutil.CmdExec(
		cmdStr,
		"Populate the reprepro's options config file with values from irgsh's config.yml",
		logPath,
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	cmdStr = fmt.Sprintf("cd %s/%s/ && reprepro -v -v -v export",
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename,
	)
	_, err = systemutil.CmdExec(
		cmdStr,
		"Initialize the reprepro repository for the first time",
		logPath,
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	fmt.Println("##### Initializing the experimental repository for " + irgshConfig.Repo.DistCodename)
	// With -experimental suffix

	cmdStr = fmt.Sprintf("mkdir -p %s && rm -rf %s/%s; cp -R %s %s/%s",
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename+"-experimental",
		repoTemplatePath,
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename+"-experimental",
	)
	_, err = systemutil.CmdExec(cmdStr, "Preparing reprepro template", logPath)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	cmdStr = fmt.Sprintf(`cd %s/%s/conf && cat updates.orig | 
		sed 's/UPSTREAM_NAME/%s/g' | 
		sed 's/UPSTREAM_DIST_CODENAME/%s/g' | 
		sed 's/UPSTREAM_DIST_URL/%s/g' | 
		sed 's/DIST_SUPPORTED_ARCHITECTURES/%s/g' | 
		sed 's/UPSTREAM_DIST_COMPONENTS/%s/g' > updates && rm updates.orig`,
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename+"-experimental",
		irgshConfig.Repo.UpstreamName,
		irgshConfig.Repo.UpstreamDistCodename+"-experimental",
		strings.Replace(irgshConfig.Repo.UpstreamDistUrl, "/", "\\/", -1),
		irgshConfig.Repo.DistSupportedArchitectures,
		irgshConfig.Repo.UpstreamDistComponents,
	)
	_, err = systemutil.CmdExec(
		cmdStr,
		"Populate the reprepro's updates config file with values from irgsh's config.yml",
		logPath,
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	cmdStr = fmt.Sprintf(`cd %s/%s/conf && cat distributions.orig |
		sed 's/DIST_NAME/%s/g' |
		sed 's/DIST_LABEL/%s/g' |
		sed 's/DIST_CODENAME/%s/g' |
		sed 's/DIST_COMPONENTS/%s/g' |
		sed 's/DIST_SUPPORTED_ARCHITECTURES/%s/g' |
		sed 's/DIST_VERSION_DESC/%s/g' |
		sed 's/DIST_VERSION/%s/g' |
		sed 's/DIST_SIGNING_KEY/%s/g' |
		sed 's/UPSTREAM_NAME/%s/g'> distributions && rm distributions.orig`,
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename+"-experimental",
		irgshConfig.Repo.DistName,
		irgshConfig.Repo.DistLabel,
		irgshConfig.Repo.DistCodename+"-experimental",
		irgshConfig.Repo.DistComponents,
		irgshConfig.Repo.DistSupportedArchitectures,
		irgshConfig.Repo.DistVersionDesc,
		irgshConfig.Repo.DistVersion,
		irgshConfig.Repo.DistSigningKey,
		irgshConfig.Repo.UpstreamName,
	)
	_, err = systemutil.CmdExec(
		cmdStr,
		"Populate the reprepro's distributions config file with values from irgsh's config.yml",
		logPath,
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	repositoryPath = strings.Replace(
		irgshConfig.Repo.Workdir+"/"+irgshConfig.Repo.DistCodename+"-experimental",
		"/",
		"\\/",
		-1,
	)
	cmdStr = fmt.Sprintf(`cd %s/%s/conf && \
	cat options.orig | sed 's/IRGSH_REPO_WORKDIR/%s/g' > options && \
	rm options.orig`,
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename+"-experimental",
		repositoryPath,
	)
	_, err = systemutil.CmdExec(
		cmdStr,
		"Populate the reprepro's options config file with values from irgsh's config.yml",
		logPath,
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	cmdStr = fmt.Sprintf("cd %s/%s/ && reprepro -v -v -v export",
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename+"-experimental",
	)
	_, err = systemutil.CmdExec(
		cmdStr,
		"Initialize the reprepro repository for the first time",
		logPath,
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	return
}

func UpdateRepo() (err error) {
	fmt.Printf("Syncing irgshConfig.Repo.against %s at %s...",
		irgshConfig.Repo.UpstreamDistCodename,
		irgshConfig.Repo.UpstreamDistUrl,
	)

	logPath := irgshConfig.Repo.Workdir + "/update.log"
	go systemutil.StreamLog(logPath)

	cmdStr := fmt.Sprintf("cd %s/%s/ && reprepro -v -v -v update > %s",
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename,
		logPath,
	)
	_, err = systemutil.CmdExec(cmdStr, "Sync the repository against upstream repository", logPath)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	cmdStr = fmt.Sprintf("cd %s/%s/ && reprepro -v -v -v export",
		irgshConfig.Repo.Workdir,
		irgshConfig.Repo.DistCodename,
	)
	_, err = systemutil.CmdExec(
		cmdStr,
		"Initialize the reprepro repository for the first time",
		logPath,
	)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	return
}
