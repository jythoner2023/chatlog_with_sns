package model

import (
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
)

const contactLabelFieldNumber = 30

func ParseContactLabelIDs(extraBuffer []byte) []int {
	if len(extraBuffer) == 0 {
		return nil
	}

	var raw string
	for len(extraBuffer) > 0 {
		fieldNum, wireType, tagLen := protowire.ConsumeTag(extraBuffer)
		if tagLen < 0 {
			return nil
		}
		extraBuffer = extraBuffer[tagLen:]

		switch wireType {
		case protowire.VarintType:
			_, valueLen := protowire.ConsumeVarint(extraBuffer)
			if valueLen < 0 {
				return nil
			}
			extraBuffer = extraBuffer[valueLen:]
		case protowire.Fixed32Type:
			_, valueLen := protowire.ConsumeFixed32(extraBuffer)
			if valueLen < 0 {
				return nil
			}
			extraBuffer = extraBuffer[valueLen:]
		case protowire.Fixed64Type:
			_, valueLen := protowire.ConsumeFixed64(extraBuffer)
			if valueLen < 0 {
				return nil
			}
			extraBuffer = extraBuffer[valueLen:]
		case protowire.BytesType:
			value, valueLen := protowire.ConsumeBytes(extraBuffer)
			if valueLen < 0 {
				return nil
			}
			if fieldNum == contactLabelFieldNumber {
				raw = string(value)
			}
			extraBuffer = extraBuffer[valueLen:]
		default:
			return nil
		}
	}

	if raw == "" {
		return nil
	}

	labelIDs := make([]int, 0)
	seen := make(map[int]struct{})
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		id, err := strconv.Atoi(item)
		if err != nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		labelIDs = append(labelIDs, id)
	}

	return labelIDs
}

func ResolveContactLabels(labelIDs []int, labelNames map[int]string) []string {
	if len(labelIDs) == 0 || len(labelNames) == 0 {
		return nil
	}

	labels := make([]string, 0, len(labelIDs))
	seen := make(map[string]struct{})
	for _, id := range labelIDs {
		label := strings.TrimSpace(labelNames[id])
		if label == "" {
			continue
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		labels = append(labels, label)
	}

	return labels
}

func (c *Contact) MatchTags(tags []string, matchAll bool) bool {
	if len(tags) == 0 {
		return true
	}

	available := make(map[string]struct{}, len(c.LabelIDs)+len(c.Labels))
	for _, id := range c.LabelIDs {
		available[strconv.Itoa(id)] = struct{}{}
	}
	for _, label := range c.Labels {
		available[strings.ToLower(strings.TrimSpace(label))] = struct{}{}
	}

	if len(available) == 0 {
		return false
	}

	matches := 0
	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized == "" {
			continue
		}
		_, ok := available[normalized]
		if matchAll && !ok {
			return false
		}
		if ok {
			matches++
			if !matchAll {
				return true
			}
		}
	}

	if matchAll {
		return matches == len(tags)
	}
	return false
}

func NormalizeTagMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), "any") {
		return "any"
	}
	return "all"
}

func ContactLabelsString(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	cloned := append([]string(nil), labels...)
	slices.Sort(cloned)
	return strings.Join(cloned, " | ")
}
