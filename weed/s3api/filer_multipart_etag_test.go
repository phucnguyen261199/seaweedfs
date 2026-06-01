package s3api

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/seaweedfs/seaweedfs/weed/filer"
	"github.com/seaweedfs/seaweedfs/weed/pb/filer_pb"
)

func TestMultipartPartETagUsesContentMD5OverInternalChunks(t *testing.T) {
	partMD5 := md5.Sum([]byte("one hadoop-aws multipart upload part split into seaweed chunks"))
	entry := &filer_pb.Entry{
		Name: "0001.part",
		Attributes: &filer_pb.FuseAttributes{
			FileSize: 64,
			Md5:      partMD5[:],
		},
		Chunks: []*filer_pb.FileChunk{
			testMultipartETagChunk([]byte("seaweed chunk 1")),
			testMultipartETagChunk([]byte("seaweed chunk 2")),
		},
	}

	internalChunkETag := filer.ETagChunks(entry.Chunks)
	contentETag := fmt.Sprintf("%x", partMD5)
	if internalChunkETag == contentETag {
		t.Fatalf("test setup failed: internal chunk ETag %q should differ from content ETag %q", internalChunkETag, contentETag)
	}

	match, invalid, normalizedPartETag, normalizedEntryETag := validateCompletePartETag(contentETag, entry)
	if invalid || !match {
		t.Fatalf("validateCompletePartETag(%q) = match %v invalid %v normalized part %q entry %q; want a valid match", contentETag, match, invalid, normalizedPartETag, normalizedEntryETag)
	}
	if normalizedEntryETag != contentETag {
		t.Fatalf("normalized entry ETag = %q, want content MD5 ETag %q instead of internal chunk ETag %q", normalizedEntryETag, contentETag, internalChunkETag)
	}
}

func TestCalculateMultipartETagUsesPartContentMD5s(t *testing.T) {
	part1MD5 := md5.Sum([]byte("part 1 content"))
	part2MD5 := md5.Sum([]byte("part 2 content large enough to be split internally"))
	partEntries := map[int][]*filer_pb.Entry{
		1: {testMultipartETagEntry("0001.part", part1MD5[:], []string{"part 1 seaweed chunk"})},
		2: {testMultipartETagEntry("0002.part", part2MD5[:], []string{"part 2 seaweed chunk 1", "part 2 seaweed chunk 2"})},
	}

	combined := append([]byte{}, part1MD5[:]...)
	combined = append(combined, part2MD5[:]...)
	want := fmt.Sprintf("%x-2", md5.Sum(combined))
	if got := calculateMultipartETag(partEntries, []int{1, 2}); got != want {
		t.Fatalf("calculateMultipartETag() = %q, want %q", got, want)
	}
}

func testMultipartETagEntry(name string, contentMD5 []byte, chunkContents []string) *filer_pb.Entry {
	chunks := make([]*filer_pb.FileChunk, 0, len(chunkContents))
	var offset int64
	for _, content := range chunkContents {
		chunk := testMultipartETagChunk([]byte(content))
		chunk.Offset = offset
		offset += int64(chunk.Size)
		chunks = append(chunks, chunk)
	}
	return &filer_pb.Entry{
		Name: name,
		Attributes: &filer_pb.FuseAttributes{
			FileSize: uint64(offset),
			Md5:      append([]byte(nil), contentMD5...),
		},
		Chunks: chunks,
	}
}

func testMultipartETagChunk(content []byte) *filer_pb.FileChunk {
	sum := md5.Sum(content)
	return &filer_pb.FileChunk{
		FileId: fmt.Sprintf("1,%x", sum[:4]),
		Size:   uint64(len(content)),
		ETag:   base64.StdEncoding.EncodeToString(sum[:]),
	}
}
