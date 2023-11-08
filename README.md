# private-rpm-repo

An example of server-less yum repository using AWS Lambda and S3.

- [AWS Lambda + S3 を使って yum レポジトリを作った](https://shogo82148.github.io/blog/2021/02/21/private-yum-repo-on-s3/)

## INSTALL

### Amazon Linux 2023

```
dnf install https://shogo82148-rpm-repository.s3.ap-northeast-1.amazonaws.com/amazonlinux/2023/noarch/shogo82148/shogo82148-1.0.6-1.amzn2023.noarch.rpm
```

### Amazon Linux 2

```
yum install https://shogo82148-rpm-repository.s3.ap-northeast-1.amazonaws.com/amazonlinux/2/noarch/shogo82148/shogo82148-1.0.7-1.amzn2.noarch.rpm
```

### AlmaLinux 9

```
dnf install https://shogo82148-rpm-repository.s3.ap-northeast-1.amazonaws.com/almalinux/9/noarch/shogo82148/shogo82148-1.0.6-1.el9.noarch.rpm
```

### AlmaLinux 8

```
yum install https://shogo82148-rpm-repository.s3.ap-northeast-1.amazonaws.com/almalinux/8/noarch/shogo82148/shogo82148-1.0.6-1.el8.noarch.rpm
```

### Rocky Linux 9

```
dnf install https://shogo82148-rpm-repository.s3.ap-northeast-1.amazonaws.com/rockylinux/9/noarch/shogo82148/shogo82148-1.0.6-1.el9.noarch.rpm
```

### Rocky Linux 8

```
yum install https://shogo82148-rpm-repository.s3.ap-northeast-1.amazonaws.com/rockylinux/8/noarch/shogo82148/shogo82148-1.0.6-1.el8.noarch.rpm
```

### CentOS 8

```
yum install https://shogo82148-rpm-repository.s3.ap-northeast-1.amazonaws.com/centos/8/noarch/shogo82148/shogo82148-1.0.1-1.el8.noarch.rpm
```

## REFERENCES

- https://github.com/rpm-software-management/createrepo_c
- https://github.com/gpg/gnupg/blob/master/doc/DETAILS
- https://mag.osdn.jp/14/01/10/090000
