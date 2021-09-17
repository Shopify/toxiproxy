package toxiproxy

type ToxicOptions struct {
	ProxyName,
	ToxicName,
	ToxicType,
	Stream string
	Toxicity   float32
	Attributes Attributes
}
