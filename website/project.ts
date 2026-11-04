import config from '../mygo.json'

// The client and the download site describe the same distribution.
export const github = config.updates.github
export const repo = `https://github.com/${github}`
export const appName = config.name
