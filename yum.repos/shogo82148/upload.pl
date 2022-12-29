#!/usr/bin/env perl

use utf8;
use strict;
use warnings;
use FindBin;
use File::Basename;

sub execute {
    my @arg = @_;
    my $cmd = join " ", @arg;
    print "executing: $cmd\n";
    my $ret = system(@arg);
    if ($ret != 0) {
        print STDERR "::warning::failed to execute $cmd";
    }
}

sub package_name {
    my $file = shift;
    my $name = basename $file;
    $name =~ s/-[0-9]+\.[0-9]+\.[0-9]+-[0-9]+\..*$//;
    return $name;
}

sub upload {
    my ($variant, $prefix) = @_;
    while (my $rpm = <$FindBin::Bin/$variant.build/RPMS/noarch/*.noarch.rpm>) {
        my $package = package_name($rpm);
        execute("aws", "s3", "cp", $rpm, "s3://shogo82148-rpm-temporary/$prefix/noarch/$package/");
        execute("aws", "s3", "cp", $rpm, "s3://shogo82148-rpm-temporary/$prefix/x86_64/$package/");
        execute("aws", "s3", "cp", $rpm, "s3://shogo82148-rpm-temporary/$prefix/aarch64/$package/");
    }
}

upload "amazonlinux2", "amazonlinux/2";
upload "amazonlinux2022", "amazonlinux/2022";
upload "centos7", "centos/7";
upload "almalinux8", "almalinux/8";
upload "almalinux9", "almalinux/9";
upload "rockylinux8", "rockylinux/8";
upload "rockylinux9", "rockylinux/9";
