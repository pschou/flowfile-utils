#!/bin/bash
  cp readme.tmpl README.md
  sed -i -e '/FF-Sender Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-sender -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-Stager Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-stager -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-Unstager Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-unstager -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-Receiver Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-receiver -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-Diode Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-diode -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-HTTP-TO-UDP Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-http-to-udp -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-UDP-TO-HTTP Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-udp-to-http -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-Sink Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-sink -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-Flood Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-flood -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-HTTP-TO-KCP Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-http-to-kcp -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-KCP-TO-HTTP Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-kcp-to-http -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-SOCKET Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-socket -h 2>&1;echo "\`\`\`")
	mv README.md ..
