for n in `find . -name "*.go"`; do
   if [[ $n != ./vendor/* ]];
   then
     go tool vet $n
     gofmt -w $n
    #  gometalinter $n
    #  golint $n
    #  gocyclo -over 15 $n
     misspell $n
   fi
done