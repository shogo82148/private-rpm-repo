# private-rpm-repo

An example of server-less yum repository using AWS Lambda and S3.

- [AWS Lambda + S3 を使って yum レポジトリを作った](https://shogo82148.github.io/blog/2021/02/21/private-yum-repo-on-s3/)

## INSTALL

### Amazon Linux 2023

```bash
dnf install https://shogo82148-rpm-repository.s3.ap-northeast-1.amazonaws.com/amazonlinux/2023/noarch/shogo82148/shogo82148-1.0.8-1.amzn2023.noarch.rpm
```

### Amazon Linux 2

```bash
yum install https://shogo82148-rpm-repository.s3.ap-northeast-1.amazonaws.com/amazonlinux/2/noarch/shogo82148/shogo82148-1.0.8-1.amzn2.noarch.rpm
```

### AlmaLinux 10

```bash
dnf install https://shogo82148-rpm-repository.s3.ap-northeast-1.amazonaws.com/almalinux/10/noarch/shogo82148/shogo82148-1.0.8-1.el10.noarch.rpm
```

### AlmaLinux 9

```bash
dnf install https://shogo82148-rpm-repository.s3.ap-northeast-1.amazonaws.com/almalinux/9/noarch/shogo82148/shogo82148-1.0.8-1.el9.noarch.rpm
```

### AlmaLinux 8

```bash
yum install https://shogo82148-rpm-repository.s3.ap-northeast-1.amazonaws.com/almalinux/8/noarch/shogo82148/shogo82148-1.0.8-1.el8.noarch.rpm
```

### Rocky Linux 10

```bash
dnf install https://shogo82148-rpm-repository.s3.ap-northeast-1.amazonaws.com/rockylinux/10/noarch/shogo82148/shogo82148-1.0.8-1.el10.noarch.rpm
```

### Rocky Linux 9

```bash
dnf install https://shogo82148-rpm-repository.s3.ap-northeast-1.amazonaws.com/rockylinux/9/noarch/shogo82148/shogo82148-1.0.8-1.el9.noarch.rpm
```

### Rocky Linux 8

```bash
yum install https://shogo82148-rpm-repository.s3.ap-northeast-1.amazonaws.com/rockylinux/8/noarch/shogo82148/shogo82148-1.0.8-1.el8.noarch.rpm
```

## REFERENCES

- https://github.com/rpm-software-management/createrepo_c
- https://github.com/gpg/gnupg/blob/master/doc/DETAILS
- https://mag.osdn.jp/14/01/10/090000
