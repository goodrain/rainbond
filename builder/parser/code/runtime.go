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
	"io/ioutil"
	"path"

	"github.com/bitly/go-simplejson"

	"github.com/goodrain/rainbond/util"
)

//CheckRuntime CheckRuntime
func CheckRuntime(buildPath string, lang Lang) bool {
	switch lang {
	case PHP:
		if ok, _ := util.FileExists(path.Join(buildPath, "composer.json")); !ok {
			return false
		}
		body, err := ioutil.ReadFile(path.Join(buildPath, "composer.json"))
		if err != nil {
			return false
		}
		json, err := simplejson.NewJson(body)
		if err != nil {
			return false
		}
		if json.Get("require") != nil && json.Get("require").Get("php") != nil {
			return true
		}
		return false
	case Python:
		if ok, _ := util.FileExists(path.Join(buildPath, "runtime.txt")); ok {
			//TODO:check runtime rules : python-2.7.3
			return true
		}
		return false

	case Ruby:
		return true
	case JavaMaven, JaveWar, JavaJar:
		ok, err := util.FileExists(path.Join(buildPath, "system.properties"))
		if !ok || err != nil {
			return false
		}
		cmd := fmt.Sprintf(`grep -i "java.runtime.version" %s | grep  -E -o "[0-9]+(.[0-9]+)?(.[0-9]+)?"`, path.Join(buildPath, "system.properties"))
		runtime, err := util.CmdExec(cmd)
		if err != nil {
			return false
		}
		if runtime != "" {
			return true
		}
		return false
	case Nodejs:
		return false
	default:
		return false
	}
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
