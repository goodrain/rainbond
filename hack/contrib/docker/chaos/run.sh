#!/bin/bash

cd $((dirname $0))
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
    return 11
  }

  which docker-compose &> /dev/null || {
    eprint 'Not found docker-compose command!'
    return 13
  }
}

start(){
  docker-compose -f docker-compose.yaml up -d
}

stop(){
  docker-compose -f docker-compose.yaml down -d
}

main(){
  check::dependency || exit $?

  eval "$cmd"
}


main
