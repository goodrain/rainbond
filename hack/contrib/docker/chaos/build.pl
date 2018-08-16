#!/usr/bin/perl
use Getopt::Long;
my $APPEND_ENV_STRING="";
my $docker_bin = -w "/var/run/docker.sock" ? "docker" : "sudo -P docker";

GetOptions(
    "name|n=s" => \$name,
    "branch|b=s" => \$branch,
    "source|s=s" => \$source_dir,
    "cache|c=s"   => \$cache_dir,
    "version|v=s"  => \$version,
    "dest|d=s"  => \$slug_dir,
    "log|l=s"   => \$logfile,
    "tenant_id|tid=s" => \$tenant_id,
    "service_id|sid=s" => \$service_id,
    "envs|e=s" => \$envs,
    "server_type|st=s" => \$server_type
);

if ($envs) {
    @envs = split(':::', $envs);
    @envs = map (quotemeta($_), @envs);
    $docker_envs = join(' -e ', @envs);
    $APPEND_ENV_STRING = "-e $docker_envs";
}
chdir($source_dir);
#system("git archive master | docker run -i --rm -v $cache_dir:/tmp/cache:rw -a stdin -a stdout goodrain.net/builder - >$package_file");
#system("git archive master | docker run -i --rm -a stdin -a stdout -e SLUG_VERSION=$version -v $slug_dir:/tmp/slug -v $cache_dir:/tmp/cache goodrain.me/builder local >$logfile");
if ($server_type eq "svn"){
    $cmd="tar -c --exclude=.svn ./ | $docker_bin run -i --net=host --rm --name $name -v $cache_dir:/tmp/cache:rw -a stdin -a stdout $APPEND_ENV_STRING -e SLUG_VERSION=$version -e SERVICE_ID=$service_id -e TENANT_ID=$tenant_id -v $slug_dir:/tmp/slug goodrain.me/builder local";
    system($cmd);
}elsif ($server_type eq "git"){
    $cmd="git archive $branch | $docker_bin run -i --net=host --rm --name $name -v $cache_dir:/tmp/cache:rw -a stdin -a stdout $APPEND_ENV_STRING -e SLUG_VERSION=$version -e SERVICE_ID=$service_id -e TENANT_ID=$tenant_id -v $slug_dir:/tmp/slug goodrain.me/builder local";
    system($cmd);
}

