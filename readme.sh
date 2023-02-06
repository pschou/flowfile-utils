#!/bin/bash
  cp readme.tmpl README.md
  sed -i -e '/NiFi-Sender Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";./nifi-sender -h 2>&1;echo "\`\`\`")
  sed -i -e '/NiFi-Stager Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";./nifi-stager -h 2>&1;echo "\`\`\`")
  sed -i -e '/NiFi-Unstager Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";./nifi-unstager -h 2>&1;echo "\`\`\`")
  sed -i -e '/NiFi-Reciever Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";./nifi-receiver -h 2>&1;echo "\`\`\`")
  sed -i -e '/NiFi-Diode Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";./nifi-diode -h 2>&1;echo "\`\`\`")
