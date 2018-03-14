// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform
 
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.
 
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
 
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package code

import (
	"fmt"
	"os"
	"os/exec"
)

//CheckRuntime CheckRuntime
func CheckRuntime(buildPath string,lang Lang) bool {
	switch lang {
	case PHP:
		//TODO: JQ??
		return false
	case Python:
		rf, bl := FileExist(buildPath, "runtime.txt")
		if ! bl {
			return false
		}
		cmd := fmt.Sprintf(`grep -i python %s | grep -E -o "[0-9]+(.[0-9]+)?(.[0-9]+)?"`, rf)
		runtime, err := CmdExec(cmd)
		if err != nil {
			return false
		}
		if runtime != "" {
			return true
		}
		return false
	case Ruby:
		return true
	case JavaMaven, JaveWar:
		rf, bl := FileExist(buildPath, "system.properties")
		if ! bl {
			return false
		}	
		cmd := fmt.Sprintf(`grep -i "java.runtime.version" %s | grep  -E -o "[0-9]+(.[0-9]+)?(.[0-9]+)?"`, rf)
		runtime, err := CmdExec(cmd)
		if err != nil {
			return false
		}
		if runtime != "" {
			return true
		}
		return false
	case Nodejs:
		//TODO: JQ？
		return false
	default:
		return false
	}
}

//FileExist FileExist
func FileExist(buildPath, filename string) (string, bool) {
	rf := fmt.Sprintf("%s/%s", buildPath, filename)
	_, err := os.Stat(rf)
	if err != nil {
		return rf, false
	}
	return rf, true
}

//CmdExec CmdExec
func CmdExec(args string) (string, error){
	out, err := exec.Command("bash", "-c", args).Output()
	if err != nil {
			return "", err
	}
	return string(out), nil
}


/*
function detect_runtimes(){
	lang=`echo $1 |tr A-Z a-z`
	case $lang in
	"php")
	  if [ -f $SOURCE_DIR/composer.json ];then
		runtimes=`$JQBIN '.require.php' $SOURCE_DIR/composer.json`
		[ "$runtimes" != "null" ] && echo "true" || echo "false"
	  else
		echo "false"
	  fi
	;;
	"python")
	  if [ -f $SOURCE_DIR/runtime.txt ];then
		runtimes=`grep -i python $SOURCE_DIR/runtime.txt | grep -E -o "[0-9]+(.[0-9]+)?(.[0-9]+)?"`
		[  "$runtimes" != "" ] && echo "true" || echo "false"
	  else
		echo "false"
	  fi
	;;
	"ruby")
  #    if [ -f $SOURCE_DIR/Gemfile ];then
  #      runtimes=`grep -E -i "^\ *ruby" $SOURCE_DIR/Gemfile | grep -E -o "[0-9]+(.[0-9]+)?(.[0-9]+)?"`
  #      [ "$runtimes" != "" ] && echo "true" || echo "false"
  #    else
  #      echo "false"
  #    fi
	   echo "true"
	;;
	"java-war|java-maven")
	  if [ -f $SOURCE_DIR/system.properties ];then
		runtimes=`grep -i "java.runtime.version" $SOURCE_DIR/system.properties | grep  -E -o "[0-9]+(.[0-9]+)?(.[0-9]+)?"`
		[ "$runtimes" != "" ] && echo "true" || echo "false"
	  else
		echo "false"
	  fi
	;;
	"node.js")
	  if [ -f $SOURCE_DIR/package.json ] ;then
		runtimes=`$JQBIN '.engines.node' $SOURCE_DIR/package.json`
		[ "$runtimes" != "null" ] && echo "true" || echo "false"
	  else
		echo "false"
	  fi
	;;
	"*")
	  echo "false";;
	esac
  }
*/