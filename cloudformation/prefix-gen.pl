#!/usr/bin/env perl

use v5.44;
use utf8;

my $package = shift or die "Usage: $0 <package>\n";
my @distributions = (
  "amazonlinux/2027",
  "amazonlinux/2023",
  "almalinux/10",
  "almalinux/9",
  "almalinux/8",
  "rockylinux/10",
  "rockylinux/9",
  "rockylinux/8",
);

my @archs = (
  "x86_64",
  "aarch64",
  "noarch",
);

say "upload prefixes:";
for my $distribution (@distributions) {
  for my $arch (@archs) {
    say "- arn:aws:s3:::shogo82148-rpm-temporary/$distribution/$arch/$package/*";
  }
}

say "";
say "list prefixes:";
for my $distribution (@distributions) {
  for my $arch (@archs) {
    say "- $distribution/$arch/$package/*";
  }
}
