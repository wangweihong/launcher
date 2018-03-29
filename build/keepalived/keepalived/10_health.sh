#!/bin/sh

containers="${CONTAINERS_TO_CHECK}"

check_state() {

	for i in \$containers
	do

		con=\$(docker ps -a | grep \$i | awk '{print \$1}' | head -n 1 | sed 's/\n//g')

		if [ \${#con} -eq 0 ]; then
			echo "UNKNOWN - \$i does not exist."
			exit 3
		fi

		RUNNING=\$(docker inspect --format="{{ .State.Running }}" \$con 2> /dev/null | sed 's/\n//g')

		if [ \$? -gt 1 ]; then
  			echo "UNKNOWN - \$i does not exist."
			exit 3
		fi

		if [ "\$RUNNING" = "false" ]; then
			echo "CRITICAL - \$i is not running."
  			exit 2
		fi

		if [ "\$RUNNING" = "true" ]; then
  			echo "OK - \$i is running."
		fi

	done

	exit 0
}

check_state
