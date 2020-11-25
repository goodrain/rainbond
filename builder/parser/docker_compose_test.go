// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package parser

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/goodrain/rainbond/event"

	"github.com/docker/docker/client"
	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
)

var dockercompose = `
version: '2'

services:
  redis:
    restart: always
    image: sameersbn/redis:latest
    command:
    - --loglevel warning
    volumes:
    - /srv/docker/gitlab/redis:/var/lib/redis
    ports:
      - "6379"

  postgresql:
    restart: always
    image: sameersbn/postgresql:9.5-3
    volumes:
    - /srv/docker/gitlab/postgresql:/var/lib/postgresql
    environment:
    - DB_USER=gitlab
    - DB_PASS=password
    - DB_NAME=gitlabhq_production
    - DB_EXTENSION=pg_trgm
    ports:
      - "5432"

  gitlab:
    restart: always
    image: sameersbn/gitlab:8.13.3
    depends_on:
    - redis
    - postgresql
    ports:
    - "10080:80"
    - "10022:22"
    volumes:
    - /srv/docker/gitlab/gitlab:/home/git/data
    environment:
    - DEBUG=false

    - DB_ADAPTER=postgresql
    - DB_HOST=postgresql
    - DB_PORT=5432
    - DB_USER=gitlab
    - DB_PASS=password
    - DB_NAME=gitlabhq_production

    - REDIS_HOST=redis
    - REDIS_PORT=6379

    - TZ=Asia/Kolkata
    - GITLAB_TIMEZONE=Kolkata

    - GITLAB_HTTPS=false
    - SSL_SELF_SIGNED=false

    - GITLAB_HOST=localhost
    - GITLAB_PORT=10080
    - GITLAB_SSH_PORT=10022
    - GITLAB_RELATIVE_URL_ROOT=
    - GITLAB_SECRETS_DB_KEY_BASE=long-and-random-alphanumeric-string
    - GITLAB_SECRETS_SECRET_KEY_BASE=long-and-random-alphanumeric-string
    - GITLAB_SECRETS_OTP_KEY_BASE=long-and-random-alphanumeric-string

    - GITLAB_ROOT_PASSWORD=
    - GITLAB_ROOT_EMAIL=

    - GITLAB_NOTIFY_ON_BROKEN_BUILDS=true
    - GITLAB_NOTIFY_PUSHER=false

    - GITLAB_EMAIL=notifications@example.com
    - GITLAB_EMAIL_REPLY_TO=noreply@example.com
    - GITLAB_INCOMING_EMAIL_ADDRESS=reply@example.com

    - GITLAB_BACKUP_SCHEDULE=daily
    - GITLAB_BACKUP_TIME=01:00

    - SMTP_ENABLED=false
    - SMTP_DOMAIN=www.example.com
    - SMTP_HOST=smtp.gmail.com
    - SMTP_PORT=587
    - SMTP_USER=mailer@example.com
    - SMTP_PASS=password
    - SMTP_STARTTLS=true
    - SMTP_AUTHENTICATION=login

    - IMAP_ENABLED=false
    - IMAP_HOST=imap.gmail.com
    - IMAP_PORT=993
    - IMAP_USER=mailer@example.com
    - IMAP_PASS=password
    - IMAP_SSL=true
    - IMAP_STARTTLS=false

    - OAUTH_ENABLED=false
    - OAUTH_AUTO_SIGN_IN_WITH_PROVIDER=
    - OAUTH_ALLOW_SSO=
    - OAUTH_BLOCK_AUTO_CREATED_USERS=true
    - OAUTH_AUTO_LINK_LDAP_USER=false
    - OAUTH_AUTO_LINK_SAML_USER=false
    - OAUTH_EXTERNAL_PROVIDERS=

    - OAUTH_CAS3_LABEL=cas3
    - OAUTH_CAS3_SERVER=
    - OAUTH_CAS3_DISABLE_SSL_VERIFICATION=false
    - OAUTH_CAS3_LOGIN_URL=/cas/login
    - OAUTH_CAS3_VALIDATE_URL=/cas/p3/serviceValidate
    - OAUTH_CAS3_LOGOUT_URL=/cas/logout

    - OAUTH_GOOGLE_API_KEY=
    - OAUTH_GOOGLE_APP_SECRET=
    - OAUTH_GOOGLE_RESTRICT_DOMAIN=

    - OAUTH_FACEBOOK_API_KEY=
    - OAUTH_FACEBOOK_APP_SECRET=

    - OAUTH_TWITTER_API_KEY=
    - OAUTH_TWITTER_APP_SECRET=

    - OAUTH_GITHUB_API_KEY=
    - OAUTH_GITHUB_APP_SECRET=
    - OAUTH_GITHUB_URL=
    - OAUTH_GITHUB_VERIFY_SSL=

    - OAUTH_GITLAB_API_KEY=
    - OAUTH_GITLAB_APP_SECRET=

    - OAUTH_BITBUCKET_API_KEY=
    - OAUTH_BITBUCKET_APP_SECRET=

    - OAUTH_SAML_ASSERTION_CONSUMER_SERVICE_URL=
    - OAUTH_SAML_IDP_CERT_FINGERPRINT=
    - OAUTH_SAML_IDP_SSO_TARGET_URL=
    - OAUTH_SAML_ISSUER=
    - OAUTH_SAML_LABEL="Our SAML Provider"
    - OAUTH_SAML_NAME_IDENTIFIER_FORMAT=urn:oasis:names:tc:SAML:2.0:nameid-format:transient
    - OAUTH_SAML_GROUPS_ATTRIBUTE=
    - OAUTH_SAML_EXTERNAL_GROUPS=
    - OAUTH_SAML_ATTRIBUTE_STATEMENTS_EMAIL=
    - OAUTH_SAML_ATTRIBUTE_STATEMENTS_NAME=
    - OAUTH_SAML_ATTRIBUTE_STATEMENTS_FIRST_NAME=
    - OAUTH_SAML_ATTRIBUTE_STATEMENTS_LAST_NAME=

    - OAUTH_CROWD_SERVER_URL=
    - OAUTH_CROWD_APP_NAME=
    - OAUTH_CROWD_APP_PASSWORD=

    - OAUTH_AUTH0_CLIENT_ID=
    - OAUTH_AUTH0_CLIENT_SECRET=
    - OAUTH_AUTH0_DOMAIN=

    - OAUTH_AZURE_API_KEY=
    - OAUTH_AZURE_API_SECRET=
    - OAUTH_AZURE_TENANT_ID=
    labels:
      kompose.service.type: NodePort  
`

var dockercompose3 = `
version: '3'
services:

  redis:
    image: redis
    restart: always

  mongo:
    image: mongo
    restart: always
    ports:
      - "27017:27017"
    volumes:
      - ~/.container/data/mongo/db:/data/db
      - ~/.container/data/mongo/configdb:/data/configdb
    environment:
      - MONGO_INITDB_ROOT_USERNAME=$MONGO_INITDB_ROOT_USERNAME
      - MONGO_INITDB_ROOT_PASSWORD=$MONGO_INITDB_ROOT_PASSWORD

  treasure-island:
    depends_on:
      - mongo
      - redis
    image: di94sh/treasure-island
    restart: always
    ports:
      - "4000:4000"
    links:
      - "mongo"
      - "redis"
    environment:
      - MONGO_INITDB_ROOT_USERNAME=$MONGO_INITDB_ROOT_USERNAME
      - MONGO_INITDB_ROOT_PASSWORD=$MONGO_INITDB_ROOT_PASSWORD

  celery:
    depends_on:
      - mongo
      - redis
    image: di94sh/treasure-island
    restart: always
    links:
      - "mongo"
      - "redis"
    environment:
      - MONGO_INITDB_ROOT_USERNAME=$MONGO_INITDB_ROOT_USERNAME
      - MONGO_INITDB_ROOT_PASSWORD=$MONGO_INITDB_ROOT_PASSWORD
    command: celery -B -A  app.tasks worker
`

var dockercompose20 = `
version: '2.0'

services:
  nginx:
    restart: always
    image: nginx:1.11.6-alpine
    ports:
      - 8080:80
      - 80:80
      - 443:443
    volumes:
      - ./conf.d:/etc/nginx/conf.d
      - ./log:/var/log/nginx
      - ./www:/var/www
      - /etc/letsencrypt:/etc/letsencrypt
`

var dockerInput = `version: '2.0'\r\nservices:\r\n  db:\r\n    image: mysql:latest\r\n    ports:\r\n      - 3306:3306\r\n    volumes:\r\n      - ./wp-data:/docker-entrypoint-initdb.d\r\n    environment:\r\n      MYSQL_DATABASE: wordpress\r\n      MYSQL_ROOT_PASSWORD: password`

//var composeJ = `{"version": "2.0","services": {"db": {"image": "mysql:latest","ports": ["3306:3306"],"volumes": ["./wp-data:/docker-entrypoint-initdb.d"],"environment": {"MYSQL_DATABASE": "wordpress","MYSQL_ROOT_PASSWORD": "password"}}}}`

var mmJ = "{\"services\": {\"db\": {\"environment\": {\"MYSQL_ROOT_PASSWORD\": \"password\", \"MYSQL_DATABASE\": \"wordpress\"}, \"image\": \"mysql:latest\", \"ports\": [\"3306:3306\"], \"volumes\": [\"./wp-data:/docker-entrypoint-initdb.d\"]}}, \"version\": \"2.0\"}"
var composeJ = `{"version": "2.0","services": {"db": {"image": "mysql:latest","ports": ["3306:3306"],"volumes": ["./wp-data:/docker-entrypoint-initdb.d"],"environment": {"MYSQL_DATABASE": "wordpress","MYSQL_ROOT_PASSWORD": "password"}}}}`

func TestDockerComposeParse(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	dockerclient, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}
	y, err := yaml.JSONToYAML([]byte(composeJ))
	if err != nil {
		fmt.Printf("yaml error, %v", err.Error())
	}
	fmt.Printf("yaml is %s", string(y))
	p := CreateDockerComposeParse(string(y), dockerclient, "", "", nil)
	if err := p.Parse(); err != nil {
		logrus.Errorf(err.Error())
		return
	}
	fmt.Printf("ServiceInfo:%+v \n", p.GetServiceInfo())
}

func TestDockerCompose30Parse(t *testing.T) {
	dockerclient, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}
	p := CreateDockerComposeParse(dockercompose3, dockerclient, "", "", event.GetTestLogger())
	if err := p.Parse(); err != nil {
		logrus.Errorf(err.Error())
		return
	}
	fmt.Printf("ServiceInfo:%+v \n", p.GetServiceInfo())
}

var fanyy = `
version: "2"
services:
  DOClever:
    image: lw96/doclever
    restart: always
    container_name: "DOClever"
    ports:
    - 10000:10000
    volumes:
    - /root/doclever/data/file:/root/DOClever/data/file
    - /root/doclever/data/img:/root/DOClever/data/img
    - /root/doclever/data/tmp:/root/DOClever/data/tmp
    environment:
    - DB_HOST=mongodb://mongo:27017/DOClever
    - PORT=10000
    links:
    - mongo:mongo

  mongo:
    image: mongo:latest
    restart: always
    container_name: "mongodb"
    ports:
    - 27017:27017
    volumes:
    - /root/doclever/data/db:/data/db
`

func TestDockerComposefanyy(t *testing.T) {
	dockerclient, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}
	p := CreateDockerComposeParse(fanyy, dockerclient, "", "", event.GetTestLogger())
	if err := p.Parse(); err != nil {
		logrus.Errorf(err.Error())
		return
	}
	svsInfos := p.GetServiceInfo()
	ss, _ := json.Marshal(svsInfos)
	fmt.Printf("ServiceInfo:%+v \n", string(ss))
}
