package types

import "encoding/json"

func jsonMarshalRows(rows []BondedSetRow) ([]byte, error) {
	return json.Marshal(rows)
}

func unmarshalRows(b []byte, m *QueryBondedSetResponse) error {
	var rows []BondedSetRow
	if err := json.Unmarshal(b, &rows); err != nil {
		return err
	}
	m.Rows = rows
	m.raw = append([]byte(nil), b...)
	return nil
}
