#!/bin/bash
  cp readme.tmpl README.md
  sed -i -e '/FF-Sender Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-sender -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-Stager Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-stager -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-Unstager Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-unstager -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-Receiver Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-receiver -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-Diode Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-diode -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-HTTP2UDP Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-http2udp -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-UDP2HTTP Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-udp2http -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-Sink Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-sink -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-Flood Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-flood -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-HTTP2KCP Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-http2kcp -h 2>&1;echo "\`\`\`")
  sed -i -e '/FF-KCP2HTTP Usage:/r /dev/stdin' README.md < <(echo "\`\`\`";../ff-kcp2http -h 2>&1;echo "\`\`\`")
	mv README.md ..
