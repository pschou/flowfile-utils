#!/bin/bash
  cp readme.tmpl README.md
  sed -i -e '/NiFi-Sender Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../nifi-sender -h 2>&1;echo "\`\`\`")
  sed -i -e '/NiFi-Stager Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../nifi-stager -h 2>&1;echo "\`\`\`")
  sed -i -e '/NiFi-Unstager Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../nifi-unstager -h 2>&1;echo "\`\`\`")
  sed -i -e '/NiFi-Receiver Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../nifi-receiver -h 2>&1;echo "\`\`\`")
  sed -i -e '/NiFi-Diode Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../nifi-diode -h 2>&1;echo "\`\`\`")
  sed -i -e '/NiFi-to-UDP Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../nifi-to-udp -h 2>&1;echo "\`\`\`")
  sed -i -e '/UDP-to-NiFi Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../udp-to-nifi -h 2>&1;echo "\`\`\`")
  sed -i -e '/NiFi-Sink Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../nifi-sink -h 2>&1;echo "\`\`\`")
  sed -i -e '/NiFi-Flood Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../nifi-flood -h 2>&1;echo "\`\`\`")
	mv README.md ..
