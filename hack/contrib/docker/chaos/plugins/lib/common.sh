#!/usr/bin/env bash

export_env_dir() {
  env_dir=$1
  whitelist_regex=${2:-''}
  blacklist_regex=${3:-'^(PATH|GIT_DIR|CPATH|CPPATH|LD_PRELOAD|LIBRARY_PATH|JAVA_OPTS)$'}
  if [ -d "$env_dir" ]; then
    for e in $(ls $env_dir); do
      echo "$e" | grep -E "$whitelist_regex" | grep -qvE "$blacklist_regex" &&
      export "$e=$(cat $env_dir/$e)"
      :
    done
  fi
}

## SBT 0.10 allows either *.sbt in the root dir, or project/*.scala or .sbt/*.scala
detect_sbt() {
  local ctxDir=$1
  if _has_sbtFile $ctxDir || \
     _has_projectScalaFile $ctxDir || \
     _has_hiddenSbtDir $ctxDir || \
     _has_buildPropertiesFile $ctxDir ; then
    return 0
  else
    return 1
  fi
}

is_play() {
  _has_playConfig $1
}

is_sbt_native_packager() {
  local ctxDir=$1
  if [ -e "${ctxDir}"/project/plugins.sbt ]; then
    pluginVersionLine="$(grep "addSbtPlugin(.\+sbt-native-packager" "${ctxDir}"/project/plugins.sbt)"
    test -n "$pluginVersionLine"
  else
    return 1
  fi
}

_has_sbtFile() {
  local ctxDir=$1
  [ -d $ctxDir ] && test -n "$(find $ctxDir -maxdepth 1 -name '*.sbt' -print -quit)"
}

_has_projectScalaFile() {
  local ctxDir=$1
  test -d $ctxDir/project && test -n "$(find $ctxDir/project -maxdepth 1 -name '*.scala' -print -quit)"
}

_has_hiddenSbtDir() {
  local ctxDir=$1
  test -d $ctxDir/.sbt && test -n "$(find $ctxDir/.sbt -maxdepth 1 -name '*.scala' -print -quit)"
}

_has_buildPropertiesFile() {
  local ctxDir=$1
  test -e $ctxDir/project/build.properties
}

_has_playConfig() {
  local ctxDir=$1
  test -e $ctxDir/conf/application.conf
}

_has_playPluginsFile() {
  local ctxDir=$1
  test -e $ctxDir/project/plugins.sbt
}

get_scala_version() {
  local ctxDir=$1
  local sbtUserHome=$2
  local launcher=$3
  local playVersion=$4

  if [ -n "${playVersion}" ]; then
    if [ "${playVersion}" = "2.3" ] || [ "${playVersion}" = "2.4" ]; then
      # if we don't grep for the version, and instead use `sbt scala-version`,
      # then sbt will try to download the internet
      scalaVersionLine="$(grep "scalaVersion" "${ctxDir}"/build.sbt | sed -E -e 's/[ \t\r\n]//g')"
      scalaVersion=$(expr "$scalaVersionLine" : ".\+\(2\.1[0-1]\)\.[0-9]")

      if [ -n "${scalaVersion}" ]; then
        echo "$scalaVersion"
      else
        echo "2.10"
      fi
    elif [ "${playVersion}" = "2.2" ]; then
      echo '2.10'
    elif [ "${playVersion}" = "2.1" ]; then
      echo '2.10'
    elif [ "${playVersion}" = "2.0" ]; then
      echo '2.9'
    else
      echo ''
    fi
  else
    echo ''
  fi
}

get_supported_play_version() {
  local ctxDir=$1
  local sbtUserHome=$2
  local launcher=$3

  if _has_playPluginsFile $ctxDir; then
    pluginVersionLine="$(grep "addSbtPlugin(.\+play.\+sbt-plugin" "${ctxDir}"/project/plugins.sbt | sed -E -e 's/[ \t\r\n]//g')"
    pluginVersion=$(expr "$pluginVersionLine" : ".\+\(2\.[0-4]\)\.[0-9]")
    if [ "$pluginVersion" != 0 ]; then
      echo -n "$pluginVersion"
    fi
  fi
  echo ""
}

get_supported_sbt_version() {
  local ctxDir=$1
  if _has_buildPropertiesFile $ctxDir; then
    sbtVersionLine="$(grep -P '[ \t]*sbt\.version[ \t]*=' "${ctxDir}"/project/build.properties | sed -E -e 's/[ \t\r\n]//g')"
    sbtVersion=$(expr "$sbtVersionLine" : 'sbt\.version=\(0\.1[1-3]\.[0-9]\(-[a-zA-Z0-9_]*\)*\)$')
    if [ "$sbtVersion" != 0 ] ; then
      echo "$sbtVersion"
    else
      echo ""
    fi
  else
    echo ""
  fi
}

prime_ivy_cache() {
  local ctxDir=$1
  local sbtUserHome=$2
  local launcher=$3

  if is_play $ctxDir ; then
    playVersion=`get_supported_play_version ${BUILD_DIR} ${sbtUserHome} ${launcher}`
  fi
  scalaVersion=$(get_scala_version "$ctxDir" "$sbtUserHome" "$launcher" "$playVersion")

  if [ -n "$scalaVersion" ]; then
    cachePkg=" (Scala-${scalaVersion}"
    if [ -n "$playVersion" ]; then
      cachePkg="${cachePkg}, Play-${playVersion}"
    fi
    cachePkg="${cachePkg})"
  fi
  status_pending "Priming Ivy cache${cachePkg}"
  if _download_and_unpack_ivy_cache "$sbtUserHome" "$scalaVersion" "$playVersion"; then
    status_done
  else
    echo " no cache found"
  fi
}

_download_and_unpack_ivy_cache() {
  local sbtUserHome=$1
  local scalaVersion=$2
  local playVersion=$3

  baseUrl="http://lang-jvm.s3.amazonaws.com/sbt/v3/sbt-cache"
  if [ -n "$playVersion" ]; then
    ivyCacheUrl="$baseUrl-play-${playVersion}_${scalaVersion}.tar.gz"
  else
    ivyCacheUrl="$baseUrl-base.tar.gz"
  fi

  curl --silent --max-time 60 --location $ivyCacheUrl | tar xzm -C $sbtUserHome
  if [ $? -eq 0 ]; then
    mv $sbtUserHome/.sbt/* $sbtUserHome
    rm -rf $sbtUserHome/.sbt
    return 0
  else
    return 1
  fi
}

has_supported_sbt_version() {
  local ctxDir=$1
  local supportedVersion="$(get_supported_sbt_version ${ctxDir})"
  if [ "$supportedVersion" != "" ] ; then
    return 0
  else
    return 1
  fi
}

has_old_preset_sbt_opts() {
  if [ "$SBT_OPTS" = "-Xmx384m -Xss512k -XX:+UseCompressedOops" ]; then
    return 0
  else
    return 1
  fi
}

count_files() {
  local location=$1
  local pattern=$2

  if [ -d ${location} ]; then
    find ${location} -name ${pattern} | wc -l | sed 's/ //g'
  else
    echo "0"
  fi
}

detect_play_lang() {
  local appDir=$1/app

  local num_scala_files=$(count_files ${appDir} '*.scala')
  local num_java_files=$(count_files ${appDir} '*.java')

  if   [ ${num_scala_files} -gt ${num_java_files} ] ; then
    echo "Scala"
  elif [ ${num_scala_files} -lt ${num_java_files} ] ; then
    echo "Java"
  else
    echo ""
  fi
}

uses_universal_packaging() {
  local ctxDir=$1
  test -d $ctxDir/target/universal/stage/bin
}

_universal_packaging_procs() {
  local ctxDir=$1
  (cd $ctxDir; find target/universal/stage/bin -type f -executable)
}

_universal_packaging_proc_count() {
  local ctxDir=$1
  _universal_packaging_procs $ctxDir | wc -l
}

universal_packaging_default_web_proc() {
  local ctxDir=$1
  if [ $(_universal_packaging_proc_count $ctxDir) -eq 1 ]; then
    echo "web: $(_universal_packaging_procs $ctxDir) -Dhttp.port=\$PORT"
  fi
}

run_sbt()
{
  local javaVersion=$1
  local home=$2
  local launcher=$3
  local tasks=$4

  case $(ulimit -u) in
  32768) # PX Dyno
    maxSbtHeap=5220
    ;;
  *)     # 2X Dyno
    maxSbtHeap=768
    ;;
  esac

  status "Running: sbt $tasks"
  HOME="$home" sbt \
    -J-Xmx${maxSbtHeap}M \
    -J-Xms${maxSbtHeap}M \
    -J-XX:+UseCompressedOops \
    -sbt-dir $home \
    -ivy $home/.ivy2 \
    -sbt-launch-dir $home/launchers \
    -Duser.home=$home \
    -Divy.default.ivy.user.dir=$home/.ivy2 \
    -Dfile.encoding=UTF8 \
    -Dsbt.global.base=$home \
    -Dsbt.log.noformat=true \
    -no-colors -batch \
    $tasks < /dev/null 2>&1 | indent

  if [ "${PIPESTATUS[*]}" != "0 0" ]; then
    error "Failed to run sbt!
We're sorry this build is failing! If you can't find the issue in application
code, please submit a ticket so we can help: https://help.heroku.com
You can also try reverting to our legacy Scala buildpack:
$ heroku buildpacks:set https://github.com/heroku/heroku-buildpack-scala#legacy

Thanks,
Heroku"
  fi
}

cache_copy() {
  rel_dir=$1
  from_dir=$2
  to_dir=$3
  rm -rf $to_dir/$rel_dir
  if [ -d $from_dir/$rel_dir ]; then
    mkdir -p $to_dir/$rel_dir
    cp -pr $from_dir/$rel_dir/. $to_dir/$rel_dir
  fi
}
