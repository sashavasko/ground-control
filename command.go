package command

type Command struct {
    SatelliteID string
    Sequence uint64
    Payload []byte
}

func NewCommand(satelliteID string, sequence uint64, payload []byte)(Command, error){
    if satelliteID == "" {
        return Command{}, fmt.error("Satellite ID is empty")
    }
    return Command{
        SatelliteID: satelliteID,
        Sequence: sequence,
        Payload: payload
    }, nil
}