FROM amazonlinux:2.0.20240916.0

RUN yum update -y && yum install -y createrepo_c && rm -rf /var/cache/yum/* && yum clean all
