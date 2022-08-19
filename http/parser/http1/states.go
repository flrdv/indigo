package http1

type parserState uint8

const (
	eMethod parserState = iota + 1
	ePath
	ePathDecode1Char
	ePathDecode2Char
	eQuery
	eQueryDecode1Char
	eQueryDecode2Char
	eFragment
	eFragmentDecode1Char
	eFragmentDecode2Char
	eProto
	eProtoCR
	eProtoCRLF
	eProtoCRLFCR
	eHeaderKey
	eHeaderColon
	eHeaderValue
	eHeaderValueCR
	eHeaderValueCRLF
	eHeaderValueCRLFCR
	eBody
)

type chunkedBodyParserState uint8

const (
	eChunkLength1Char chunkedBodyParserState = iota + 1
	eChunkLength
	eChunkLengthCR
	eChunkLengthCRLF
	eChunkBody
	eChunkBodyCR
	eChunkBodyCRLF
	eLastChunkCR
)