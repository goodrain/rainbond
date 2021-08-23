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
  }

  which docker-compose &> /dev/null || {
    eprint 'Not found docker-compose command!'
    
    install::docker-compose || {
      eprint 'Failed to install docker-compose!'
      return 13
    }

    iprint 'successful install docker-compose!'
  }
  
  return 0
}

install::docker(){
  curl -fsSL https://get.docker.com -o get-docker.sh &&
  sh get-docker.sh &&
  which docker &>/dev/null &&
  systemctl start docker &&
  systemctl enable docker
}

install::docker-compose(){
  curl -L "https://github.com/docker/compose/releases/download/1.24.0/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
  chmod +x /usr/local/bin/docker-compose
  which docker-compose &>/dev/null
}

import::image(){
  docker load -i component-images.tar
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

  eval "$cmd"
}


main
