@Library("talkdesk-default-pipeline") _
defaultPipeline {
    name = 'toxiproxy'
    scmUrl = 'https://github.com/Talkdesk/toxiproxy'
    stgBranch = 'staging'
    qaBranch = 'qa'
    enableDR = false
    deploy = [
        [
            platforms: ['kubernetes'],
            name: 'toxiproxy',
            namespace: 'platform-edge'
        ]
    ]
}
