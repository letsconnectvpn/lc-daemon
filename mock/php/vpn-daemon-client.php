<?php

/*
 * To test:
 *
 * make
 * ./_bin/vpn-daemon &
 * php php/vpn-daemon-client.php
 */

$commandList = [
    'SET_PORTS 11940 11941',
    'LIST',
    'DISCONNECT foo bar baz',
//  'SETUP cn profile1 profile2',
    'QUIT',
];

$socket = stream_socket_client('tcp://localhost:41194');

foreach ($commandList as $cmd) {
    var_dump(sendCommand($socket, $cmd));
}

/*
$localCommandList = [
    'CLIENT_CONNECT profile1 cn 1234567890 10.52.58.2 fdbf:4dff:a892:1572::1000'
    'CLIENT_DISCONNECT profile1 cn 1234567890 10.52.58.2 fdbf:4dff:a892:1572::1000 605666 9777056 120'
];

$localSocket = stream_socket_client('tcp://localhost:41195');

foreach ($localCommandList as $cmd) {
    var_dump(sendCommand($localSocket, $cmd));
}
*/

function sendCommand($socket, $cmd)
{
    fwrite($socket, sprintf("%s\n", $cmd));

    return handleResponse($socket);
}

function handleResponse($socket)
{
    $statusLine = fgets($socket);
    if (0 !== strpos($statusLine, 'OK: ')) {
        echo $statusLine;
        exit(1);
    }
    $resultLineCount = (int) substr($statusLine, 4);
    $resultData = [];
    for ($i = 0; $i < $resultLineCount; $i++) {
        $resultData[] = trim(fgets($socket));
    }
   
    return $resultData;
}
