package server

// inputSchema describes the text and file request, including exclusive question variants and closed defined objects.
const inputSchema = `{
  "type": "object",
  "properties": {
    "objects": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string",
            "minLength": 1
          },
          "source": {
            "oneOf": [
              {
                "type": "object",
                "properties": {"type": {"const": "text"}, "text": {"type": "string"}},
                "required": ["type", "text"],
                "additionalProperties": false
              },
              {
                "type": "object",
                "properties": {
                  "type": {"const": "file"},
                  "path": {"type": "string"},
                  "lines": {
                    "type": "object",
                    "properties": {"start": {"type": "integer"}, "end": {"type": "integer"}},
                    "required": ["start", "end"],
                    "additionalProperties": false
                  }
                },
                "required": ["type", "path"],
                "additionalProperties": false
              }
            ]
          }
        },
        "required": [
          "id",
          "source"
        ],
        "additionalProperties": false
      }
    },
    "task": {
      "type": "string",
      "minLength": 1
    },
    "questions": {
      "type": "object",
      "additionalProperties": {
        "oneOf": [
          {
            "type": "object",
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "choice"
                ]
              },
              "instructions": {
                "anyOf": [
                  {
                    "type": "string"
                  },
                  {
                    "type": "object"
                  },
                  {
                    "type": "array"
                  }
                ]
              },
              "criteria": {
                "type": "object",
                "additionalProperties": {
                  "anyOf": [
                    {
                      "type": "string"
                    },
                    {
                      "type": "object"
                    },
                    {
                      "type": "array"
                    },
                    {
                      "type": "null"
                    }
                  ]
                },
                "minProperties": 1,
                "propertyNames": {
                  "type": "string",
                  "minLength": 1
                }
              }
            },
            "required": [
              "type",
              "instructions",
              "criteria"
            ],
            "additionalProperties": false
          },
          {
            "type": "object",
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "noul"
                ]
              },
              "instructions": {
                "anyOf": [
                  {
                    "type": "string"
                  },
                  {
                    "type": "object"
                  },
                  {
                    "type": "array"
                  }
                ]
              },
              "criteria": {
                "type": "object",
                "properties": {
                  "true": {
                    "anyOf": [
                      {
                        "type": "string"
                      },
                      {
                        "type": "object"
                      },
                      {
                        "type": "array"
                      }
                    ]
                  },
                  "false": {
                    "anyOf": [
                      {
                        "type": "string"
                      },
                      {
                        "type": "object"
                      },
                      {
                        "type": "array"
                      }
                    ]
                  }
                },
                "required": [
                  "true",
                  "false"
                ],
                "additionalProperties": false
              }
            },
            "required": [
              "type",
              "instructions"
            ],
            "additionalProperties": false
          },
          {
            "type": "object",
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "score"
                ]
              },
              "instructions": {
                "anyOf": [
                  {
                    "type": "string"
                  },
                  {
                    "type": "object"
                  },
                  {
                    "type": "array"
                  }
                ]
              },
              "criteria": {
                "type": "array",
                "items": {
                  "anyOf": [
                    {
                      "type": "string"
                    },
                    {
                      "type": "object"
                    },
                    {
                      "type": "array"
                    }
                  ]
                },
                "minItems": 1
              }
            },
            "required": [
              "type",
              "instructions",
              "criteria"
            ],
            "additionalProperties": false
          }
        ]
      },
      "minProperties": 1,
      "propertyNames": {
        "type": "string",
        "minLength": 1
      }
    },
    "result_mode": {
      "type": "string",
      "enum": [
        "compact",
        "full"
      ]
    }
  },
  "required": [
    "objects",
    "task",
    "questions"
  ],
  "additionalProperties": false
}`

// outputSchema describes success and atomic failure entries, with compact and full assessment alternatives.
const outputSchema = `{
  "type": "object",
  "properties": {
    "results": {
      "type": "array",
      "minItems": 1,
      "items": {
        "oneOf": [
          {
            "type": "object",
            "properties": {
              "id": {
                "type": "string"
              },
              "answers": {
                "type": "object",
                "minProperties": 1,
                "additionalProperties": {
                  "oneOf": [
                    {
                      "type": "object",
                      "properties": {
                        "choice": {
                          "type": "string"
                        },
                        "probability": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1
                        },
                        "confidence": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1
                        }
                      },
                      "required": [
                        "choice",
                        "probability"
                      ],
                      "additionalProperties": false
                    },
                    {
                      "type": "object",
                      "properties": {
                        "choice": {
                          "type": "string"
                        },
                        "probability": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1
                        },
                        "confidence": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1
                        },
                        "probabilities": {
                          "type": "object",
                          "minProperties": 1,
                          "additionalProperties": {
                            "type": "number",
                            "minimum": 0,
                            "maximum": 1
                          }
                        }
                      },
                      "required": [
                        "choice",
                        "probability",
                        "probabilities"
                      ],
                      "additionalProperties": false
                    },
                    {
                      "type": "object",
                      "properties": {
                        "noul": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1
                        }
                      },
                      "required": [
                        "noul"
                      ],
                      "additionalProperties": false
                    },
                    {
                      "type": "object",
                      "properties": {
                        "score": {
                          "type": "number",
                          "minimum": 0
                        },
                        "confidence": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1
                        }
                      },
                      "required": [
                        "score"
                      ],
                      "additionalProperties": false
                    },
                    {
                      "type": "object",
                      "properties": {
                        "score": {
                          "type": "number",
                          "minimum": 0
                        },
                        "confidence": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1
                        },
                        "probabilities": {
                          "type": "object",
                          "minProperties": 1,
                          "additionalProperties": {
                            "type": "number",
                            "minimum": 0,
                            "maximum": 1
                          }
                        }
                      },
                      "required": [
                        "score",
                        "probabilities"
                      ],
                      "additionalProperties": false
                    }
                  ]
                }
              }
            },
            "required": [
              "id",
              "answers"
            ],
            "additionalProperties": false
          },
          {
            "type": "object",
            "properties": {
              "id": {
                "type": "string"
              },
              "error": {
                "type": "string",
                "minLength": 1
              }
            },
            "required": [
              "id",
              "error"
            ],
            "additionalProperties": false
          }
        ]
      }
    }
  },
  "required": [
    "results"
  ],
  "additionalProperties": false
}`

// Optional assessment field names are shared by the projection serializers.
const (
	fieldConfidence    = "confidence"
	fieldProbabilities = "probabilities"
)
