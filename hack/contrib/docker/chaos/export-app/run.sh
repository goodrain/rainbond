#!/bin/bash

cd $(dirname $0)
cmd="$1"
[[ x$cmd == x ]] && cmd=start

eprint(){
  echo -e "\033[0;37;41m $* \033[0m"
}

iprint(){
  echo -e "\033[0;37;42m $* \033[0m"
}

check::dependency(){
  which docker &> /dev/null || {
    eprint 'Not found docker command!'

    install::docker || {
      eprint 'Failed to install docker!'
      return 11
    }
    
    iprint 'successful install docker!'
    return 0
  }

  which docker-compose &> /dev/null || {
    eprint 'Not found docker-compose command!'
    
    install::docker-compose || {
      eprint 'Failed to install docker-compose!'
      return 13
    }

    iprint 'successful install docker-compose!'
    return 0
  }
}

install::docker(){
  wget -O /etc/yum.repos.d/docker-ce.repo https://download.docker.com/linux/centos/docker-ce.repo &&
  yum install -y docker-ce &&
  which docker &>/dev/null &&
  systemctl start docker &&
  systemctl enable docker
}

install::docker-compose(){
  curl -L https://github.com/docker/compose/releases/download/1.21.2/docker-compose-$(uname -s)-$(uname -m) -o /usr/local/bin/docker-compose
  chmod +x /usr/local/bin/docker-compose
  which docker-compose &>/dev/null
}

import::image(){
  find . -name '*.image.tar' | xargs -I LOADIMAGES docker load -i LOADIMAGES
}

gen::config(){
  sed -i 's/""//g' docker-compose.yaml
  sed -i "s|__GROUP_DIR__|$(pwd)|g" docker-compose.yaml
  sed -i "s/\*\*None\*\*/$(uuidgen | tr -d -)/g" docker-compose.yaml
}

start(){
  import::image
  docker-compose -f docker-compose.yaml up -d
}

stop(){
  docker-compose -f docker-compose.yaml down
}

main(){
  check::dependency || exit $?
  gen::config

  eval "$cmd"
}


main
