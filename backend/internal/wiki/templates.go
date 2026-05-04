package wiki

// entityTemplates maps entity type to a suggested summary template.
// Applied in CreateEntity when the caller sends an empty summary, so the
// entity arrives pre-structured without the writer having to know the format.
// All field labels use NexusTale-native language — no third-party framework names.
var entityTemplates = map[string]string{
	"character": "Core Motivation: \n\nArc: [start state] → [end state]\n\nVoice & Presence: \n\nKey Relationships: ",
	"location":  "Description: \n\nHistory & Significance: \n\nWho lives here: \n\nConnections to plot: ",
	"faction":   "Purpose & Values: \n\nLeadership: \n\nRelationship to other factions: \n\nResources: ",
	"item":      "What it does: \n\nOrigin: \n\nWho possesses it: \n\nSignificance to the story: ",
	"concept":   "What it is: \n\nHow it works: \n\nWho knows about it: \n\nRole in the story: ",
	"lore":      "What happened: \n\nCauses: \n\nConsequences: \n\nWho was present: ",
}
