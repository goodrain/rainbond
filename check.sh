for n in `find . -name "*.go"`; do
   if [[ $n != ./vendor/* ]];
   then
     gofmt -w $n
     golint $n
   fi
done