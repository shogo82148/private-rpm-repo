FROM amazonlinux:2.0.20230119.1

RUN yum update -y && yum install -y createrepo_c && rm -rf /var/cache/yum/* && yum clean all
